package fr.plys.tv.data

import androidx.paging.PagingSource
import androidx.paging.PagingState

class CatalogPagingSource(
    private val api: PlysApi,
    private val query: CatalogQuery,
    firstPage: ItemPage,
    private val onLoaded: (Int, List<ItemCard>) -> Unit = { _, _ -> },
) : PagingSource<Int, ItemCard>() {
    private var seed: ItemPage? = firstPage

    override suspend fun load(params: LoadParams<Int>): LoadResult<Int, ItemCard> {
        val offset = params.key ?: query.offset
        return try {
            val page = seed?.takeIf { it.offset == offset }?.also { seed = null }
                ?: api.catalog(query.copy(offset = offset))
            onLoaded(offset, page.items)
            LoadResult.Page(
                data = page.items,
                prevKey = if (offset > 0) (offset - page.limit).coerceAtLeast(0) else null,
                nextKey = if (page.items.isEmpty() || offset + page.items.size >= page.total) null else offset + page.items.size,
            )
        } catch (error: ClientException) {
            LoadResult.Error(error)
        }
    }

    override fun getRefreshKey(state: PagingState<Int, ItemCard>): Int? {
        val anchor = state.anchorPosition ?: return null
        val page = state.closestPageToPosition(anchor) ?: return null
        return page.prevKey?.plus(40) ?: page.nextKey?.minus(page.data.size)
    }
}