package main

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/uber/jaeger-client-go"
	jaegercfg "github.com/uber/jaeger-client-go/config"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
)

var (
	ctx = context.Background()
	cfg jaegercfg.Configuration
)

func initJaeger() (opentracing.Tracer, io.Closer) {
	cfg = jaegercfg.Configuration{
		ServiceName: "service-a",
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

	service := os.Getenv("service")
	log.Println(service)
	r.GET("/service", func(c *gin.Context) {
		extract, _ := tracer.Extract(opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(c.Request.Header))
		span := tracer.StartSpan("call service a", ext.RPCServerOption(extract))
		defer span.Finish()
		contextWithSpan := opentracing.ContextWithSpan(context.Background(), span)
		foo3("foo3", contextWithSpan)
		foo4("foo4", contextWithSpan)

		readAll := callServiceb(contextWithSpan, service, c)
		c.JSON(http.StatusOK, gin.H{
			"data": string(readAll),
		})
	})
	r.Run("127.0.0.1:8000")
}

func callServiceb(ctx context.Context, service string, c *gin.Context) []byte {
	spanFromContext, _ := opentracing.StartSpanFromContext(ctx, "span_call_serviceb")
	defer func() {
		spanFromContext.SetTag("request", "serviceb")
		spanFromContext.SetTag("reply", "iamok")
		spanFromContext.Finish()
	}()
	request, _ := http.NewRequest(http.MethodGet, service, nil)
	ext.SpanKindRPCClient.Set(spanFromContext)
	ext.HTTPUrl.Set(spanFromContext, service)
	spanFromContext.Tracer().Inject(spanFromContext.Context(), opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(request.Header))
	client := http.Client{}
	log.Printf("%+v", request.Header)
	resp, _ := client.Do(request)
	readAll, _ := ioutil.ReadAll(resp.Body)

	return readAll
}

func foo3(function string, ctx context.Context) (reply string) {
	spanFromContext, _ := opentracing.StartSpanFromContext(ctx, "span_foo3")
	defer func() {
		spanFromContext.SetTag("request", function)
		spanFromContext.SetTag("reply", reply)
		spanFromContext.Finish()
	}()
	time.Sleep(time.Second * 2)
	reply = "foo3Replay"
	return
}
func foo4(function string, ctx context.Context) (reply string) {
	spanFromContext, _ := opentracing.StartSpanFromContext(ctx, "span_foo4")
	defer func() {
		spanFromContext.SetTag("request", function)
		spanFromContext.SetTag("reply", reply)
		spanFromContext.Finish()
	}()
	time.Sleep(time.Second)
	reply = "foo4Replay"
	return
}
