# Taken down via a DMCA request

This repository was taken down due to a DMCA request.

The copyright owners allege that this repository contained "Go code that
implements a hack of the Readium LCP DRM", which is completely false: all I did
was read [the specification](https://readium.org/lcp-specs/releases/lcp/latest)
and follow it to the letter - the code never attempted to brute force the user
key or include any secret/copyrighted material.

However, DMCA also includes a section (§1201) that prohibits "circumventing a
technological measure that effectively controls access to a copyrighted work".
Although there are plenty of legitimate use cases for `lcp-decrypt` (eg.
reading legally acquired ebooks on devices that don't support LCP), it falls in
that bucket.

I made `lcp-decrypt` public in the hope that it'd be useful to other people
like me, but I don't want to engage in a legal process in the US. My only
option therefore is to remove the code from GitHub.
