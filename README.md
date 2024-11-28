# lcp-decrypt - a quick&dirty tool to remove LCP/CARE DRMs

lcp-decrypt takes an ePUB file protected with a Readium LCP protection
(sometimes also called CARE) and decrypts it into a regular ePUB file. The
decryption requires the LCP user key. The retrieval of the key is not handled
by this program, in other words, lcp-decrypt does not crack any DRM. It only
makes a DRMed ePUB you already have legitimate access to usable on any ePUB
compatible reader.

You can use lcp-decrypt [online](https://abustany.github.io/lcp-decrypt/) or
build it as a CLI application that you can then use locally.

## Building the lcp-decrypt CLI

Until binaries are provided, you need to compile the tool yourself by running

```
go build ./cmd/lcp-decrypt
```

## Running lcp-decrypt

Once you have your user key (as a hex encoded string), getting a decoded ePUB is as simple as running

```
# decrypts ebook_with_drm.epub into ebook_without_drm.epub
lcp-decrypt -userKey 012345 ebook_with_drm.epub ebook_without_drm.epub
```

## Retrieving the LCP user key

The process to retrieve the user key depends on how you officially access the
ebook you purchased.

### Vivlio Reader

I must give credits to Vivlio for providing a Linux version of their reader.
The reader is an Electron application, which means it's easy to tell it to
forward all requests through [mitmproxy](https://mitmproxy.org/). Assuming
mitmproxy is running on port 8080, run `./Vivlio-3.3.0.AppImage
--proxy-server=127.0.0.1:8080`. Log into your ebook reseller through the app
and open the book. There should be one request in the mitmproxy console that
looks like this:

```
GET https://api.your-book-store.com/v1/lcp/keys/user?device_id=XXX
```

and the response should look like

```
[{"user_key": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}]
```

The `0123...` string is the value you should pass to the `-userKey` command
line flag.

## Limitations

As mentioned above, this is a quick&dirty tool. The ePUB parsing was tested
on the one file I have access to, and I only checked that the resulting ePUB
worked in Calibre and on a Kindle. Bug reports and contributions are welcome.
