package fr.plys.tv.ui

import android.content.Context
import fr.plys.tv.R
import fr.plys.tv.data.ClientException
import fr.plys.tv.data.Failure

fun Context.errorMessage(error: ClientException): String = when (error.failure) {
    Failure.URL -> getString(R.string.error_url)
    Failure.NETWORK -> getString(R.string.error_network)
    Failure.CREDENTIALS -> getString(R.string.error_credentials)
    Failure.SESSION -> getString(R.string.error_session)
    Failure.SETUP -> getString(R.string.error_setup)
    Failure.BUSY -> getString(R.string.error_busy)
    Failure.NOT_FOUND -> getString(R.string.error_not_found)
    Failure.STORAGE -> getString(R.string.error_storage)
    Failure.RESPONSE -> getString(R.string.error_response)
    Failure.SERVER -> getString(R.string.error_server, error.status)
}
