package main

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/uber/jaeger-client-go"
	jaegercfg "github.com/uber/jaeger-client-go/config"
	"io"
	"log"
	"net/http"
)

var (
	ctx = context.Background()
	cfg jaegercfg.Configuration
)

func initJaeger() (opentracing.Tracer, io.Closer) {
	cfg = jaegercfg.Configuration{
		ServiceName: "service-c",
		Sampler: &jaegercfg.SamplerConfig{
			Type:  jaeger.SamplerTypeConst,
			Param: 1,
		},
		Reporter: &jaegercfg.ReporterConfig{
			LogSpans:           true,
			LocalAgentHostPort: "127.0.0.1:6831",
		},
	}
	tracer, closer, err := cfg.NewTracer()
	if err != nil {
		log.Fatalln(err)
		return nil, nil
	}
	return tracer, closer
}
func main() {
	tracer, closer := initJaeger()
	defer closer.Close()
	opentracing.SetGlobalTracer(tracer)

	r := gin.Default()

	r.GET("/service", func(c *gin.Context) {
		extract, _ := tracer.Extract(opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(c.Request.Header))
		span := tracer.StartSpan("call service c",ext.RPCServerOption(extract))
		defer span.Finish()

		c.JSON(http.StatusOK, gin.H{
			"data": "iamok",
		})
	})
	r.Run("127.0.0.1:7000")
}
