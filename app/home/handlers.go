package home

import (
	"net/http"

	"github.com/qor/qor"
	"github.com/qor/qor/utils"
	"github.com/qor/render"
	"github.com/rs/zerolog/log"
)

// Controller home controller
type Controller struct {
	View *render.Render
}

// Index home index page
func (ctrl Controller) Index(w http.ResponseWriter, req *http.Request) {
	log.Info().Msg("viewpaths for home/index")
	for _, v := range ctrl.View.ViewPaths {
		log.Info().Msg(v)
	}
	ctrl.View.Execute("index", map[string]interface{}{}, req, w)
}

// SwitchLocale switch locale
func (ctrl Controller) SwitchLocale(w http.ResponseWriter, req *http.Request) {
	utils.SetCookie(http.Cookie{Name: "locale", Value: req.URL.Query().Get("locale")}, &qor.Context{Request: req, Writer: w})
	http.Redirect(w, req, req.Referer(), http.StatusSeeOther)
}
