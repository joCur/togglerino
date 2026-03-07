import { useEffect, useRef } from 'react'

export function useInfiniteScroll(options: {
  hasNextPage: boolean | undefined
  isFetchingNextPage: boolean
  fetchNextPage: () => void
}) {
  const sentinelRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const sentinel = sentinelRef.current
    if (!sentinel) return

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0].isIntersecting && options.hasNextPage && !options.isFetchingNextPage) {
          options.fetchNextPage()
        }
      },
      { threshold: 0.1 }
    )

    observer.observe(sentinel)
    return () => observer.disconnect()
  }, [options.hasNextPage, options.isFetchingNextPage, options.fetchNextPage])

  return sentinelRef
}
