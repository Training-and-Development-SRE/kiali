package config_dump

import (
	"io"
	"net/http"
	"os"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"

	"github.com/kiali/kiali/log"
)

type PortForwarder interface {
	Start() error
	Stop()
}

type forwarder struct {
	forwarder *portforward.PortForwarder
	ReadyCh   chan struct{}
	StopCh    chan struct{}
}

func (f forwarder) Start() error {
	// It starts the port-forward
	errCh := make(chan error, 1)
	go func() {
		errCh <- f.forwarder.ForwardPorts()
	}()

	// Waiting until the ReadyChan has a value
	select {
	case err := <-errCh:
		log.Errorf("Failing starting the port forwarding")
		return err
	case <-f.ReadyCh:
		// Ready to forward requests
		return nil
	}
}

func (f forwarder) Stop() {
	// Closing the StopCh channel is closing the forwarding
	close(f.StopCh)
}

func NewPortForwarder(client rest.Interface, clientConfig *rest.Config, namespace, pod, address, portMap string, writer io.Writer) (forwarder, error) {
	stopCh := make(chan struct{})
	readyCh := make(chan struct{})

	forwarderUrl := client.Post().
		Namespace(namespace).
		Resource("pods").
		Name(pod).
		SubResource("portforward").URL()

	transport, upgrader, err := spdy.RoundTripperFor(clientConfig)
	if err != nil {
		log.Errorf("Error creating a RoundTripper: %v", err)
		return forwarder{}, err
	}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, forwarderUrl)
	fwer, err := portforward.NewOnAddresses(dialer, []string{address}, []string{portMap},
		stopCh, readyCh, writer, os.Stderr)

	if err != nil {
		log.Errorf("Error creating the port-forwarder: %v", err)
		return forwarder{}, err
	}

	return forwarder{
		forwarder: fwer,
		ReadyCh:   readyCh,
		StopCh:    stopCh,
	}, nil
}
