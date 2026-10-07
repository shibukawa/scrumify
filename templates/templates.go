package templates

import (
	"net/url"

	"github.com/shibukawa/popcornweb/pw"
)

// RuntimeScriptURL backs the external declaration in document.pw.html.
//
// The runtime it names applies the sections of a page that arrive after the
// rest of it, which is what a template declaring an `async` parameter needs.
// A page without one loads it and finds nothing to do.
//
// The template calls this rather than writing a literal path, because the URL
// carries a revision derived from the script's own bytes: an upgrade that
// changes the runtime changes the URL, and a literal would go on pointing at
// bytes the server no longer serves.
func RuntimeScriptURL() *url.URL { return &url.URL{Path: pw.RuntimeScriptURL()} }

// AssetURL backs the external declaration in document.pw.html.
//
// It takes the path of a file inside the served tree — "app.css", not
// "/public/app.css" — and returns the URL this build serves it under. That URL
// carries a revision derived from the file's own bytes, so a deployment can
// answer it immutably: a browser holding it never asks again, and an edit
// changes the URL rather than the answer behind it.
//
// A literal path still works and still resolves. It just has no revision, so
// the browser revalidates it on every page load.
//
// Use it for the assets nothing else renames: a stylesheet, a plain script. An
// `img src` and a TypeScript `script src` are rewritten by the build
// already, into names that carry their own digest, and writing those through
// this function would hide them from the conversion that does it.
func AssetURL(name string) *url.URL { return &url.URL{Path: pw.PublicAssetURL(name)} }
