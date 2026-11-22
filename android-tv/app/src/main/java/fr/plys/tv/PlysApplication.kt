package fr.plys.tv

import android.app.Application
import fr.plys.tv.data.PlysApi
import fr.plys.tv.data.SecureSessionStore
import fr.plys.tv.update.AppUpdater

class PlysApplication : Application() {
    val api by lazy { PlysApi(SecureSessionStore(this)) }
    val updater by lazy { AppUpdater(this) }
}
