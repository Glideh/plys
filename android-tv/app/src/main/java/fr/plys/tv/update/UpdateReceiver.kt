package fr.plys.tv.update

import android.content.ActivityNotFoundException
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.pm.PackageInstaller
import androidx.core.content.IntentCompat
import fr.plys.tv.PlysApplication

/** Receives the status of an install session started by [AppUpdater]. */
class UpdateReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        val updater = (context.applicationContext as PlysApplication).updater
        val status = intent.getIntExtra(PackageInstaller.EXTRA_STATUS, PackageInstaller.STATUS_FAILURE)
        if (status == PackageInstaller.STATUS_PENDING_USER_ACTION) {
            val confirm = IntentCompat.getParcelableExtra(intent, Intent.EXTRA_INTENT, Intent::class.java)
            if (confirm != null) {
                try {
                    context.startActivity(confirm.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
                    return
                } catch (_: ActivityNotFoundException) {
                    // Reported below as a failed install.
                }
            }
            updater.finished(PackageInstaller.STATUS_FAILURE)
            return
        }
        updater.finished(status)
    }
}
