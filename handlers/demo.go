package handlers

import (
	"net/http"
	"sync"

	"scrumify/internal/tracker"

	"github.com/shibukawa/popcornweb/pw"
)

// RegisterDemoData installs a middleware that seeds the sample project on the
// first request of a development run. Call it from main before pw.Run; it
// does nothing outside development.
func RegisterDemoData() {
	var once sync.Once
	pw.RegisterMiddleware(pw.SlotStorage+5, "demo_data", func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// The environment is known only once pw.Run has read the
			// configuration, so the check sits here rather than at registration.
			once.Do(func() {
				if !pw.Development() {
					return
				}
				if err := tracker.EnsureDemo(pw.Context(r)); err != nil {
					pw.Logger(r).Error("demo data", pw.Err(err))
				}
			})
			next.ServeHTTP(w, r)
		})
	})
}
