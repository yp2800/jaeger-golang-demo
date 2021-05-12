package main

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/extra/redisotel/v8"
	"github.com/go-redis/redis/v8"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/uber/jaeger-client-go"
	jaegercfg "github.com/uber/jaeger-client-go/config"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

var (
	ctx = context.Background()
	cfg jaegercfg.Configuration
)

func initJaeger() (opentracing.Tracer, io.Closer) {
	cfg = jaegercfg.Configuration{
		ServiceName: "service-b",
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

	service := os.Getenv("service")
	r := gin.Default()

	r.GET("/service", func(c *gin.Context) {
		extract, _ := tracer.Extract(opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(c.Request.Header))
		span := tracer.StartSpan("call service b", ext.RPCServerOption(extract))
		defer span.Finish()
		contextWithSpan := opentracing.ContextWithSpan(context.Background(), span)

		readAll := callServicec(contextWithSpan, service, c)
		connectRedis(contextWithSpan)
		c.JSON(http.StatusOK, gin.H{
			"data": string(readAll),
		})
	})
	r.Run("127.0.0.1:9000")
}

func connectRedis(ctx context.Context) {
	spanFromContext, spanCtx := opentracing.StartSpanFromContext(ctx, "span_redis")
	defer func() {
		spanFromContext.SetTag("request", "redis")
		spanFromContext.SetTag("reply", "redisdata")
		spanFromContext.Finish()
	}()
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	defer client.Close()
	startSpanFromContext, ctx2 := opentracing.StartSpanFromContext(spanCtx, "span_connect_redis")
	defer startSpanFromContext.Finish()
	client.AddHook(redisotel.NewTracingHook())
	client.Ping(ctx2)
}

func callServicec(ctx context.Context, service string, header *gin.Context) []byte {
	spanFromContext, _ := opentracing.StartSpanFromContext(ctx, "span_call_servicec")
	defer func() {
		spanFromContext.SetTag("request", "serviceb")
		spanFromContext.SetTag("reply", "iamok")
		spanFromContext.Finish()
	}()
	request, _ := http.NewRequest(http.MethodGet, service, nil)
	ext.SpanKindRPCClient.Set(spanFromContext)
	ext.HTTPUrl.Set(spanFromContext, http.MethodGet)
	spanFromContext.Tracer().Inject(spanFromContext.Context(), opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(request.Header))
	client := http.Client{}
	resp, _ := client.Do(request)
	readAll, _ := ioutil.ReadAll(resp.Body)

	return readAll
}
