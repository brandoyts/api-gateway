package main

import (
	"api-gateway/config"
	"api-gateway/internal/proxy"
	"log"
	"net/http"
)

func main() {
	config.LoadGatewayConfiguration()
	gatewayConfiguration := config.GetGatewayConfiguration()

	proxyHandler := proxy.NewProxyHandler(gatewayConfiguration.RequestTimeout)

	for _, route := range gatewayConfiguration.Routes {
		err := proxyHandler.AddRoute(route.Prefix, route.BackendUrl)
		if err != nil {
			log.Fatalf("error adding routes: %v\n", err)
		}
		log.Printf("%v route added successfully\n", route.Prefix)
	}

	gateway := &http.Server{
		Addr:    gatewayConfiguration.ListenAddress,
		Handler: proxyHandler,
	}

	err := gateway.ListenAndServe()
	log.Fatal(err)
}
