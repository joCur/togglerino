package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func (r *Registry) Handler() http.Handler {
	return promhttp.HandlerFor(r.Prometheus, promhttp.HandlerOpts{})
}
