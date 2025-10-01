# Hardproxy

## What?

This is a simple proxy that follows redirects, rather than passing redirects back to the client. It's called `hardproxy` because it never refreshes the data once it has been cached.

## Why?

I was testing some performance issues in an RSS feed reader, and I wanted to cache the remote content to remove network latency from the picture. Most websites are only available via `https://`, and redirect `http://` requests to the secure version of the URL. Most proxies pass these redirects back the client.

This proxy follows redirects itself, so if a client requests (e.g.) <http://www.nytimes.com/services/xml/rss/nyt/National.xml>, the proxy will handle the redirect to <https://www.nytimes.com/services/xml/rss/nyt/National.xml> and cache the content, and return the result to the client. Future requests to <http://www.nytimes.com/services/xml/rss/nyt/National.xml> will be handled by the proxy cache.
