package fr.plys.tv.ui

import androidx.compose.runtime.Composable
import androidx.compose.ui.res.stringResource
import fr.plys.tv.R

/** Says that some servers were left out of an aggregated answer, naming them when the server reports their names. */
@Composable
fun partialResults(count: Int, names: List<String>): String =
    if (names.isEmpty()) stringResource(R.string.partial_results, count)
    else stringResource(R.string.partial_results_named, names.joinToString(", "))
