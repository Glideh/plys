package fr.plys.tv.update

import android.app.PendingIntent
import android.content.ActivityNotFoundException
import android.content.Context
import android.content.Intent
import android.content.pm.PackageInstaller
import android.os.Build
import android.provider.Settings
import androidx.core.content.edit
import androidx.core.net.toUri
import fr.plys.tv.BuildConfig
import java.io.File
import java.io.IOException
import java.util.concurrent.TimeUnit
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import okhttp3.OkHttpClient
import okhttp3.Request

enum class UpdateFailure { DOWNLOAD, SIGNATURE, INSTALL }

sealed interface UpdateState {
    data object None : UpdateState
    data class Available(val update: AvailableUpdate) : UpdateState
    /** [progress] is between 0 and 1, or null while the size is unknown. */
    data class Downloading(val update: AvailableUpdate, val progress: Float?) : UpdateState
    data class Installing(val update: AvailableUpdate) : UpdateState
    /** Android's "install unknown apps" setting was opened for Plys: selecting the update again continues. */
    data class NeedsPermission(val update: AvailableUpdate) : UpdateState
    data class Failed(val update: AvailableUpdate, val reason: UpdateFailure) : UpdateState
}

private const val CHECK_INTERVAL_MS = 24L * 60 * 60 * 1000
private const val PREFS = "plys-update"
private const val KEY_ETAG = "etag"
private const val KEY_BODY = "body"

/**
 * Updates the app from the GitHub releases: the latest release is read silently at launch and then once a day while
 * the app runs, with its ETag so that an unchanged release costs a 304. A newer release shows an entry in the rail;
 * selecting it downloads the APK and hands it to the system installer, which asks the user to confirm.
 */
class AppUpdater(private val context: Context) {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private val prefs = context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
    private val http = OkHttpClient.Builder()
        .connectTimeout(15, TimeUnit.SECONDS)
        .readTimeout(60, TimeUnit.SECONDS)
        .build()
    private val userAgent = "Plys-TV/${BuildConfig.VERSION_NAME}"
    private val feed = BuildConfig.UPDATE_REPO.takeIf { it.isNotBlank() }
        ?.let { ReleaseFeed(http, "https://api.github.com/repos/$it/releases/latest", userAgent) }
    private val mutableState = MutableStateFlow<UpdateState>(UpdateState.None)
    val state: StateFlow<UpdateState> = mutableState.asStateFlow()
    private var checks: Job? = null
    private var download: Job? = null
    private val updatesDir get() = File(context.cacheDir, "updates")

    /** Starts the background checks once per process; later calls do nothing. */
    @Synchronized
    fun start() {
        if (feed == null || checks != null) return
        checks = scope.launch {
            while (isActive) {
                check()
                delay(CHECK_INTERVAL_MS)
            }
        }
    }

    private fun check() {
        val result = feed?.fetch(prefs.getString(KEY_ETAG, null)) ?: return
        val body = when (result) {
            is FeedResult.Changed -> {
                prefs.edit { putString(KEY_ETAG, result.etag).putString(KEY_BODY, result.body) }
                result.body
            }
            FeedResult.NotModified -> prefs.getString(KEY_BODY, null)
            FeedResult.Unavailable -> return
        } ?: return
        val update = parseRelease(body)?.let { availableUpdate(it, BuildConfig.VERSION_NAME) }
        if (update == null) updatesDir.deleteRecursively()
        // A download or an install in progress is never interrupted by a check.
        mutableState.value = when (val current = mutableState.value) {
            is UpdateState.Downloading, is UpdateState.Installing -> current
            else -> if (update == null) UpdateState.None
                else if (current.update() == update) current else UpdateState.Available(update)
        }
    }

    /** The rail entry was selected: downloads the APK, then asks the system to install it. */
    fun install() {
        val update = mutableState.value.update() ?: return
        if (mutableState.value is UpdateState.Downloading || mutableState.value is UpdateState.Installing) return
        if (!context.packageManager.canRequestPackageInstalls()) {
            mutableState.value = UpdateState.NeedsPermission(update)
            openInstallPermission()
            return
        }
        download?.cancel()
        download = scope.launch {
            mutableState.value = UpdateState.Downloading(update, null)
            val apk = try {
                downloadApk(update)
            } catch (_: IOException) {
                mutableState.value = UpdateState.Failed(update, UpdateFailure.DOWNLOAD)
                return@launch
            }
            mutableState.value = UpdateState.Installing(update)
            try {
                commit(apk)
            } catch (_: IOException) {
                mutableState.value = UpdateState.Failed(update, UpdateFailure.INSTALL)
            } catch (_: SecurityException) {
                mutableState.value = UpdateState.Failed(update, UpdateFailure.INSTALL)
            }
        }
    }

    private fun openInstallPermission() {
        val intent = Intent(Settings.ACTION_MANAGE_UNKNOWN_APP_SOURCES, "package:${context.packageName}".toUri())
            .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
        try {
            context.startActivity(intent)
        } catch (_: ActivityNotFoundException) {
            // Some TV builds have no such screen: the hint in the rail still tells the user what to allow.
        }
    }

    private fun downloadApk(update: AvailableUpdate): File {
        val dir = updatesDir.apply { mkdirs() }
        dir.listFiles()?.forEach { it.delete() }
        val target = File(dir, "plys-${update.version}.apk")
        val partial = File(dir, "${target.name}.part")
        val request = Request.Builder().url(update.apkUrl).header("User-Agent", userAgent).build()
        http.newCall(request).execute().use { response ->
            if (!response.isSuccessful) throw IOException("HTTP ${response.code}")
            val body = response.body ?: throw IOException("empty body")
            val total = body.contentLength().takeIf { it > 0 } ?: update.size.takeIf { it > 0 }
            var written = 0L
            var shown = -1
            body.byteStream().use { input ->
                partial.outputStream().use { output ->
                    val buffer = ByteArray(64 * 1024)
                    while (true) {
                        val read = input.read(buffer)
                        if (read < 0) break
                        output.write(buffer, 0, read)
                        written += read
                        if (total != null) {
                            val percent = (written * 100 / total).toInt().coerceIn(0, 100)
                            if (percent != shown) {
                                shown = percent
                                mutableState.value = UpdateState.Downloading(update, percent / 100f)
                            }
                        }
                    }
                }
            }
            if (update.size > 0 && written != update.size) throw IOException("truncated download")
        }
        if (!partial.renameTo(target)) throw IOException("rename failed")
        return target
    }

    private fun commit(apk: File) {
        val installer = context.packageManager.packageInstaller
        val params = PackageInstaller.SessionParams(PackageInstaller.SessionParams.MODE_FULL_INSTALL)
            .apply { setAppPackageName(context.packageName) }
        val id = installer.createSession(params)
        installer.openSession(id).use { session ->
            apk.inputStream().use { input ->
                session.openWrite("plys.apk", 0, apk.length()).use { output ->
                    input.copyTo(output)
                    session.fsync(output)
                }
            }
            // Mutable: the installer adds the status and the confirmation intent to it.
            val flags = PendingIntent.FLAG_UPDATE_CURRENT or
                if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) PendingIntent.FLAG_MUTABLE else 0
            val callback = PendingIntent.getBroadcast(context, id, Intent(context, UpdateReceiver::class.java), flags)
            session.commit(callback.intentSender)
        }
    }

    /** Called by [UpdateReceiver] with the installer's final status. */
    internal fun finished(status: Int) {
        val update = mutableState.value.update() ?: return
        mutableState.value = when (status) {
            // On success Android replaces and restarts the app; cancelling the confirmation keeps the offer.
            PackageInstaller.STATUS_SUCCESS, PackageInstaller.STATUS_FAILURE_ABORTED -> UpdateState.Available(update)
            // Another signing key: the installed app was built elsewhere and has to be reinstalled by hand once.
            PackageInstaller.STATUS_FAILURE_CONFLICT, PackageInstaller.STATUS_FAILURE_INCOMPATIBLE ->
                UpdateState.Failed(update, UpdateFailure.SIGNATURE)
            else -> UpdateState.Failed(update, UpdateFailure.INSTALL)
        }
    }
}

private fun UpdateState.update(): AvailableUpdate? = when (this) {
    UpdateState.None -> null
    is UpdateState.Available -> update
    is UpdateState.Downloading -> update
    is UpdateState.Installing -> update
    is UpdateState.NeedsPermission -> update
    is UpdateState.Failed -> update
}
