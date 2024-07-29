package api

import (
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
)

type API struct {
	// API version
	Version string
	mux     *http.ServeMux
}

func NewAPI() *API {
	api := API{
		Version: "1.0",
		mux:     http.NewServeMux(),
	}
	api.mux.Handle("/metrics", promhttp.Handler())

	return &api
}

func (api *API) Serve(addr string) {
	err := http.ListenAndServe(addr, api.mux)
	if err != nil {
		panic(err)
	}
}

func (api *API) Stop() {
}
