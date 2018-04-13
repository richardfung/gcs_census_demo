package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"cloud.google.com/go/storage"
	"go.opencensus.io/exporter/stackdriver"
	"go.opencensus.io/exporter/stackdriver/propagation"
	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/trace"
	"golang.org/x/net/context"
	"google.golang.org/api/internal"
)

func main() {
	// Census setup.
	projectID := "richardfung-gcs-census"
	se, err := stackdriver.NewExporter(stackdriver.Options{ProjectID: projectID})
	if err != nil {
		log.Fatalf("StatsExporter err: %v", err)
	}
	defer se.Flush()

	trace.RegisterExporter(se)
	view.RegisterExporter(se)

	for i, v := range []*view.View{ocgrpc.ClientErrorCountView, ocgrpc.ClientRoundTripLatencyView} {
		if err := v.Subscribe(); err != nil {
			log.Printf("Views.Subscribe (#%d) err: %v", i, err)
		}
		defer v.Unsubscribe()
	}

	trace.SetDefaultSampler(trace.AlwaysSample())

	// Finished with Census setup. Now back to regular app stuff.

	body := "Hello world!"
	objectName := "firstObject"

	ctx := context.Background()

	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	object := client.Bucket("singleton").Object(objectName)
	writer := object.NewWriter(ctx)
	if n, err := writer.Write([]byte(body)); err != nil {
		log.Fatalf("Error writing: %v", err)
	} else if n != len([]byte(body)) {
		log.Println("Didn't write entire body? Got %d want %d", n, len([]byte(body)))
	}

	if err = writer.Close(); err != nil {
		log.Fatalf("Error closing writer: %v", err)
	}

	reader, err := object.NewReader(ctx)
	if err != nil {
		log.Fatalf("Error creating reader: %v", err)
	}
	defer reader.Close()

	ret, err := ioutil.ReadAll(reader)
	if err != nil {
		log.Fatalf("Error reading: %v", err)
	}
	fmt.Println(string(ret))
}

func getDialSettings() (*internal.DialSettings, error) {
	o := &internal.DialSettings{}
	opts := []option.ClientOption{
		option.WithScopes(storage.ScopeFullControl),
	}
	for _, opt := range opts {
		opt.Apply(o)
	}

	if err := o.Validate(); err != nil {
		return nil, err
	}
	return o, nil
}

// This is necessary so that we can introduce some latency into the client.
// When we pass in our own http client, some of the logic does not get run so
// we do it manually here.
//
func getHttpClient(ctx context.Context) (*http.Client, error) {
	dialSettings, err := getDialSettings()
	if err != nil {
		return nil, err
	}

	srt, err := getRoundTripper(ctx, dialSettings)
	if err != nil {
		return nil, err
	}

	return &http.Client{Transport: srt}, dialSettings.Endpoint, nil
}

func getRoundTripper(ctx context.Context, dialSettings *internal.DialSettings) (RoundTripper, error) {
	creds, err := internal.Creds(ctx, dialSettings)
	if err != nil {
		return nil, err
	}
	return &oauth2.Transport{
		Base: ochttp.Transport{
			Base:        NewSlowRoundTripper(),
			Propagation: &propagation.HTTPFormat{},
		},
		Source: creds.TokenSource,
	}, nil
}

type SlowRoundTripper struct {
	base RoundTripper
}

func NewSlowRoundTripper() *SlowRoundTripper {
	return &SlowRoundTripper{base: http.DefaultTransport}
}

func (srt *SlowRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	time.sleep(5 * time.Second)
	return base.RoundTrip(req)
}
