package main

import (
	"flag"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	ops int

	timeout   time.Duration
	verbose   bool
	namespace string
	promaddr  string

	OperationCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "kubernoisy",
		Name:      "action_count_total",
		Help:      "Counter of object actions",
	}, []string{"object", "action"})

	ValidationFailCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "kubernoisy",
		Name:      "validation_fail_count_total",
		Help:      "Counter of validation failures",
	}, []string{"action"})

	ValidationDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "kubernoisy",
		Name:      "validation_duration_seconds",
		Buckets:   prometheus.LinearBuckets(0, 1, 30), // from 0.1s to 8 seconds
		Help:      "Delay to reflect in DNS record",
	}, []string{"action"})
)

func main() {
	flag.IntVar(&ops, "ops", 1, "Operations per second")
	flag.StringVar(&promaddr, "prom", ":9696", "Prometheus endpoint")
	flag.StringVar(&namespace, "namespace", "load-test", "Namespace to operate in")
	flag.DurationVar(&timeout, "timeout", 30*time.Second, "Timeout for validation")
	flag.BoolVar(&verbose, "verbose", false, "Verbose log output")

	flag.Parse()

	if ops <= 0 {
		log.Fatal("ops cannot be <= 0")
	}

	// listen for signals
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	// get k8s api connection
	kapi, err := getAPIConn()
	if err != nil {
		log.Fatal(err)
	}

	// serve prometheus metrics
	http.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(promaddr, nil)

	// start ops ticker
	ticker := time.NewTicker(1 * time.Second / time.Duration(ops))
	defer ticker.Stop()

	log.Printf("Performing %v operations per second", ops)
	for {
		select {
		case <-ticker.C:
			go func() {
				// generate unique name
				rando := "kubernoisy-" + RandStringBytes(18)

				// create headless service
				svc := &v1.Service{
					ObjectMeta: metav1.ObjectMeta{Name: rando, Namespace: namespace},
					Spec: v1.ServiceSpec{
						Ports:     []v1.ServicePort{{Name: "http", Port: 80}},
						ClusterIP: v1.ClusterIPNone,
						Type:      v1.ServiceTypeClusterIP,
					},
				}
				svc, err := kapi.CoreV1().Services(namespace).Create(svc)
				if err != nil {
					log.Printf("could not create service %v.%v: %v", rando, namespace, err)
				}
				OperationCount.WithLabelValues("service", "add").Inc()

				// create endpoints
				ip := "1.2.3.4"
				eps := &v1.Endpoints{
					ObjectMeta: metav1.ObjectMeta{Name: rando, Namespace: namespace},
					Subsets: []v1.EndpointSubset{{
						Addresses: []v1.EndpointAddress{{IP: ip, Hostname: "test"}},
						Ports:     []v1.EndpointPort{{Name: "http", Port: 80}},
					}},
				}
				eps, err = kapi.CoreV1().Endpoints(namespace).Create(eps)
				if err != nil {
					debugf("could not create endpoints %v.%v: %v", rando, namespace, err)
				}
				OperationCount.WithLabelValues("endpoints", "add").Inc()

				// verify via DNS in loop with timeout
				verified := false
				var elapsed time.Duration
				for start := time.Now(); time.Since(start) < timeout; {
					ips, err := net.LookupIP(rando)
					if err == nil && len(ips) > 0 && ips[0].String() == ip {
						verified = true
						break
					}
					time.Sleep(time.Second)
					elapsed = time.Since(start)
				}
				if !verified {
					ValidationFailCount.WithLabelValues("add").Inc()
				} else {
					ValidationDuration.WithLabelValues("add").Observe(elapsed.Seconds())
				}

				// delete headless service and (implicitly) endpoints
				err = kapi.CoreV1().Services(namespace).Delete(rando, &metav1.DeleteOptions{})
				if err != nil {
					debugf("could not delete service %v.%v: %v", rando, namespace, err)
				}
				OperationCount.WithLabelValues("service", "delete").Inc()

				// verify via DNS in loop with timeout
				verified = false
				elapsed = 0
				for start := time.Now(); time.Since(start) < timeout; {
					_, err := net.LookupIP(rando)
					if err != nil && strings.Contains(err.Error(), "no such host") {
						verified = true
						break
					}
					time.Sleep(time.Second)
					elapsed = time.Since(start)
				}
				if !verified {
					ValidationFailCount.WithLabelValues("delete").Inc()
				} else {
					ValidationDuration.WithLabelValues("delete").Observe(elapsed.Seconds())
				}
			}()
		case <-sig:
			log.Printf("Got signal, exiting")
			os.Exit(0)
		}
	}
}

func debugf(fmt string, v ...interface{}) {
	if !verbose {
		return
	}
	log.Printf(fmt, v...)
}

func getAPIConn() (*kubernetes.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	config.ContentType = "application/vnd.kubernetes.protobuf"

	return kubernetes.NewForConfig(config)
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func RandStringBytes(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

const letterBytes = "0123456789abcdefghijklmnopqrstuvwxyz"
