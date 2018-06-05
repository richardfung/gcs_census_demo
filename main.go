package main

import (
	"fmt"
	"io/ioutil"
	"log"

	"cloud.google.com/go/storage"
	"go.opencensus.io/exporter/stackdriver"
	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/trace"
	"golang.org/x/net/context"
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

	if err := view.Register(ocgrpc.DefaultClientViews...); err != nil {
		log.Fatal(err)
	}

	trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})

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
