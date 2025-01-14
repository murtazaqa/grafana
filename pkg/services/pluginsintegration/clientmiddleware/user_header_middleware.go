package clientmiddleware

import (
	"context"
	"net/http"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	sdkhttpclient "github.com/grafana/grafana-plugin-sdk-go/backend/httpclient"
	"github.com/grafana/grafana/pkg/infra/httpclient/httpclientprovider"
	"github.com/grafana/grafana/pkg/plugins"
	"github.com/grafana/grafana/pkg/services/contexthandler"
	"github.com/grafana/grafana/pkg/util/proxyutil"
)

// NewUserHeaderMiddleware creates a new plugins.ClientMiddleware that will
// populate the X-Grafana-User header on outgoing plugins.Client and HTTP
// requests.
func NewUserHeaderMiddleware() plugins.ClientMiddleware {
	return plugins.ClientMiddlewareFunc(func(next plugins.Client) plugins.Client {
		return &UserHeaderMiddleware{
			next: next,
		}
	})
}

type UserHeaderMiddleware struct {
	next plugins.Client
}

func (m *UserHeaderMiddleware) applyToken(ctx context.Context, pCtx backend.PluginContext, h backend.ForwardHTTPHeaders) context.Context {
	reqCtx := contexthandler.FromContext(ctx)
	// if no HTTP request context skip middleware
	if h == nil || reqCtx == nil || reqCtx.Req == nil || reqCtx.SignedInUser == nil {
		return ctx
	}

	h.DeleteHTTPHeader(proxyutil.UserHeaderName)
	if !reqCtx.IsAnonymous {
		h.SetHTTPHeader(proxyutil.UserHeaderName, reqCtx.Login)
	}

	middlewares := []sdkhttpclient.Middleware{}

	if !reqCtx.IsAnonymous {
		httpHeaders := http.Header{
			proxyutil.UserHeaderName: []string{reqCtx.Login},
		}

		middlewares = append(middlewares, httpclientprovider.SetHeadersMiddleware(httpHeaders))
	} else {
		middlewares = append(middlewares, httpclientprovider.DeleteHeadersMiddleware(proxyutil.UserHeaderName))
	}

	ctx = sdkhttpclient.WithContextualMiddleware(ctx, middlewares...)

	return ctx
}

func (m *UserHeaderMiddleware) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	if req == nil {
		return m.next.QueryData(ctx, req)
	}

	ctx = m.applyToken(ctx, req.PluginContext, req)

	return m.next.QueryData(ctx, req)
}

func (m *UserHeaderMiddleware) CallResource(ctx context.Context, req *backend.CallResourceRequest, sender backend.CallResourceResponseSender) error {
	if req == nil {
		return m.next.CallResource(ctx, req, sender)
	}

	ctx = m.applyToken(ctx, req.PluginContext, req)

	return m.next.CallResource(ctx, req, sender)
}

func (m *UserHeaderMiddleware) CheckHealth(ctx context.Context, req *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	if req == nil {
		return m.next.CheckHealth(ctx, req)
	}

	ctx = m.applyToken(ctx, req.PluginContext, req)

	return m.next.CheckHealth(ctx, req)
}

func (m *UserHeaderMiddleware) CollectMetrics(ctx context.Context, req *backend.CollectMetricsRequest) (*backend.CollectMetricsResult, error) {
	return m.next.CollectMetrics(ctx, req)
}

func (m *UserHeaderMiddleware) SubscribeStream(ctx context.Context, req *backend.SubscribeStreamRequest) (*backend.SubscribeStreamResponse, error) {
	return m.next.SubscribeStream(ctx, req)
}

func (m *UserHeaderMiddleware) PublishStream(ctx context.Context, req *backend.PublishStreamRequest) (*backend.PublishStreamResponse, error) {
	return m.next.PublishStream(ctx, req)
}

func (m *UserHeaderMiddleware) RunStream(ctx context.Context, req *backend.RunStreamRequest, sender *backend.StreamSender) error {
	return m.next.RunStream(ctx, req, sender)
}
