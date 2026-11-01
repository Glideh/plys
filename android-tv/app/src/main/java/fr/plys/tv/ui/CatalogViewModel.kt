package fr.plys.tv.ui

import android.app.Application
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import fr.plys.tv.PlysApplication
import fr.plys.tv.data.ClientException
import fr.plys.tv.data.ItemDetail
import fr.plys.tv.data.ItemPage
import fr.plys.tv.data.Library
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

sealed interface Screen {
    data object Login : Screen
    data class Libraries(val items: List<Library>) : Screen
    data class Catalog(val library: Library, val page: ItemPage) : Screen
    data class Detail(val item: ItemDetail) : Screen
}

data class CatalogState(
    val screen: Screen = Screen.Login,
    val loading: Boolean = false,
    val error: ClientException? = null,
    val canRetry: Boolean = true,
    val playback: ItemDetail? = null,
    val playbackPosition: Double = 0.0,
)

class CatalogViewModel(application: Application) : AndroidViewModel(application) {
    val api = (application as PlysApplication).api
    private val mutableState = MutableStateFlow(CatalogState())
    val state = mutableState.asStateFlow()
    private val history = mutableListOf<Screen>()
    private var retryAction: suspend () -> Unit = { restore() }

    init { perform { restore() } }

    private suspend fun restore() {
        if (api.signedIn || api.restore()) {
            mutableState.value = mutableState.value.copy(screen = Screen.Libraries(api.libraries()))
        }
    }

    fun login(server: String, username: String, password: String) = perform(retryable = false) {
        api.login(server, username, password)
        mutableState.value = mutableState.value.copy(screen = Screen.Libraries(api.libraries()))
    }

    fun logout() = perform {
        api.logout()
        history.clear()
        mutableState.value = mutableState.value.copy(screen = Screen.Login)
    }

    fun openLibrary(library: Library) = perform {
        navigate(Screen.Catalog(library, api.items(library.id, 0)))
    }

    fun page(offset: Int) {
        val screen = mutableState.value.screen as? Screen.Catalog ?: return
        perform {
            mutableState.value = mutableState.value.copy(screen = Screen.Catalog(
                screen.library, api.items(screen.library.id, offset),
            ))
        }
    }

    fun openItem(id: String) = perform { navigate(Screen.Detail(api.item(id))) }

    fun play(item: ItemDetail, restart: Boolean = false) = perform {
        val target = item.playTarget
        val playable = if (target != null) api.item(target.id) else item
        if (playable.playableVersion == null) {
            throw ClientException(fr.plys.tv.data.Failure.NOT_FOUND)
        }
        mutableState.value = mutableState.value.copy(
            playback = playable,
            playbackPosition = if (restart) 0.0 else target?.positionSeconds ?: playable.positionSeconds,
        )
    }

    fun closePlayer() {
        mutableState.value = mutableState.value.copy(playback = null)
        val screen = mutableState.value.screen
        if (screen is Screen.Detail) perform {
            mutableState.value = mutableState.value.copy(screen = Screen.Detail(api.item(screen.item.id)))
        }
    }

    fun retry() = perform(action = retryAction)

    fun back(): Boolean {
        if (mutableState.value.loading) return true
        if (history.isEmpty()) return false
        val screen = history.removeAt(history.lastIndex)
        mutableState.value = mutableState.value.copy(screen = screen, error = null)
        perform {
            val refreshed = when (screen) {
                Screen.Login -> screen
                is Screen.Libraries -> Screen.Libraries(api.libraries())
                is Screen.Catalog -> Screen.Catalog(screen.library, api.items(screen.library.id, screen.page.offset))
                is Screen.Detail -> Screen.Detail(api.item(screen.item.id))
            }
            mutableState.value = mutableState.value.copy(screen = refreshed)
        }
        return true
    }

    val canGoBack: Boolean get() = history.isNotEmpty()

    private fun navigate(screen: Screen) {
        history.add(mutableState.value.screen)
        mutableState.value = mutableState.value.copy(screen = screen)
    }

    private fun perform(retryable: Boolean = true, action: suspend () -> Unit) {
        if (mutableState.value.loading) return
        if (retryable) retryAction = action
        mutableState.value = mutableState.value.copy(loading = true, error = null, canRetry = retryable)
        viewModelScope.launch {
            try {
                action()
            } catch (error: ClientException) {
                if (!api.signedIn) {
                    history.clear()
                    mutableState.value = mutableState.value.copy(screen = Screen.Login)
                } else if (mutableState.value.screen == Screen.Login) {
                    retryAction = { restore() }
                    mutableState.value = mutableState.value.copy(screen = Screen.Libraries(emptyList()), canRetry = true)
                }
                mutableState.value = mutableState.value.copy(error = error)
            } finally {
                mutableState.value = mutableState.value.copy(loading = false)
            }
        }
    }
}
