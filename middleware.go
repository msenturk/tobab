package main

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/sirupsen/logrus"
)

func (app *Tobab) getRBACMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			h := r.Host
			u, err := app.extractUser(r)
			if err != nil && err != ErrUnauthenticatedRequest {
				//this shouldn't happen unless someone tampered with a cookie manually
				app.logger.WithError(err).Error("Unable to extract user")
				//invalid cookie is present, delete it and force re-auth
				c := http.Cookie{
					Name:     "X-Tobab-Token",
					Domain:   app.config.CookieScope,
					SameSite: http.SameSiteLaxMode,
					Secure:   true,
					HttpOnly: true,
					MaxAge:   -1,
					Path:     "/",
				}
				http.SetCookie(w, &c)
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			app.logger.WithFields(logrus.Fields{
				"host": h,
				"user": u,
				"uri":  r.RequestURI,
			}).Debug("checking auth")

			if !hasAccess(u, h, app.config) {
				if err == ErrUnauthenticatedRequest {
					redirectURL := url.URL{
						Host:   h,
						Path:   r.URL.String(),
						Scheme: "https",
					}
					c := http.Cookie{
						Domain:   app.config.CookieScope,
						Secure:   true,
						HttpOnly: true,
						Value:    redirectURL.String(),
						Path:     "/",
						Name:     "X-Tobab-Source",
					}
					http.SetCookie(w, &c)
					http.Redirect(w, r, app.fqdn, 302)
				} else {
					http.Error(w, "access denied", http.StatusUnauthorized)
				}

				return
			}

			r.Header.Add("X-Tobab-User", u)

			if h != app.config.Hostname {
				//get all cookies, clear them, and then re-add the ones that are not tobab specific
				cookies := r.Cookies()
				r.Header.Del("Cookie")
				for _, c := range cookies {
					if !strings.HasPrefix(c.Name, "X-Tobab") {
						r.AddCookie(c)
					}
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}
