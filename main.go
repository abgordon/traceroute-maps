package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

const (
	geoAPIKey       = "<your-key>"
	googleMapAPIKey = "<your-key>"
)

var htmlTemplate = `<html>
  <head>
    <meta name="viewport" content="initial-scale=1.0, user-scalable=no">
    <meta charset="utf-8">
    <title>Simple Polylines</title>
    <style>
      /* Always set the map height explicitly to define the size of the div
       * element that contains the map. */
      #map {
        height: 100%%;
      }
      /* Optional: Makes the sample page fill the window. */
      html, body {
        height: 100%%;
        margin: 0;
        padding: 0;
      }
    </style>
  </head>
  <body>
    <div id="map"></div>
    <script>
      function initMap() {
        var map = new google.maps.Map(document.getElementById('map'), {
          zoom: 4,
          center: {lat: 33.5917265, lng: -106.2022343},
          mapTypeId: 'terrain'
        });

        var flightPlanCoordinates = [ %s ]; // this is where iptrace inputs lat / long

        var flightPath = new google.maps.Polyline({
          path: flightPlanCoordinates,
          geodesic: true,
          strokeColor: '#FF0000',
          strokeOpacity: 1.0,
          strokeWeight: 2
        });

        flightPath.setMap(map);
      }
    </script>
    <script async defer
    src="https://maps.googleapis.com/maps/api/js?key=%s&callback=initMap">
    </script>
  </body>
</html>`

type ip struct {
	IP            string  `json:"ip"`
	City          string  `json:"city"`
	Region        string  `json:"region"`
	RegionCode    string  `json:"region_code"`
	CountryName   string  `json:"country_name"`
	CountryCode   string  `json:"country_code"`
	ContinentName string  `json:"continent_name"`
	ContinentCode string  `json:"continent_code"`
	Latitude      float64 `json:"latitude"`
	Longitude     float64 `json:"longitude"`
	ASN           *ASN    `json:"asn"`
	Organisation  string  `json:"organisation"`
}

type ASN struct {
	ASN       string `json:"asn"`
	Name      string `json:"name"`
	Domain    string `json:"domain"`
	RouteCIDR string `json:"route"`
	Type      string `json:"type"`
}

func main() {

	args := os.Args

	if len(args) == 1 {
		fmt.Println("no host found; exiting. Provide a host via 'go run main.go <hostname>'")
		os.Exit(1)
	}

	fmt.Printf("running 'traceroute -n %s'....\n", args[1])
	out, err := exec.Command("traceroute", "-n", "google.com").Output()
	if err != nil {
		fmt.Println("err:", err.Error())
	}

	outStr := string(out)
	fmt.Printf("traceroute: \n%s\n\n", outStr)

	newLnSplit := strings.Split(outStr, "\n")
	hosts := []string{}
	for _, l := range newLnSplit {
		spl := strings.Split(l, " ")
		if len(spl) >= 4 {
			// this is fkn boneheaded. Can traceroute do host-only output?
			hosts = append(hosts, resolveHost(spl))
		}
	}

	// geolocate
	latLonPairs := [][]float64{}
	for _, h := range hosts {
		location, err := resolveLocation(h)
		if err != nil {
			fmt.Printf("error retrieving geolocation; skipping location on route: %v\n", err)
			continue
		}

		latLonPairs = append(latLonPairs, location)
	}

	// construct obj string google map expects
	str := ""
	for i, v := range latLonPairs {
		if v[0] != 0 && v[1] != 0 {
			str += fmt.Sprintf("{ lat: %v, lng: %v }", v[0], v[1])
			if i != len(latLonPairs)-1 {
				str += ", "
			}
		}
	}

	html := fmt.Sprintf(htmlTemplate, str, googleMapAPIKey)
	err = ioutil.WriteFile("index.html", []byte(html), 0644)
	if err != nil {
		fmt.Printf("error writing to index.html, exiting: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Traceroute render complete and written to index.html. Run 'php -S localhost:8080' to view in a browser.\n")
}

func resolveHost(spl []string) string {
	switch {
	case spl[0] == "" && spl[1] != "":
		// [3]
		return spl[3]
	case spl[0] != "" && spl[1] == "":
		// [2]
		return spl[2]
	case spl[0] == "" && spl[1] == "":
		// [4]
		return spl[4]
	}
	return ""
}

// resolve lat,long as 2-tuples
func resolveLocation(ipAddr string) ([]float64, error) {
	fmt.Printf("retrieving ip %s....", ipAddr)
	url := fmt.Sprintf("https://api.ipdata.co/%s?api-key=%s", ipAddr, geoAPIKey)
	resp, err := http.Get(url)
	if err != nil {
		return []float64{}, err
	}

	defer resp.Body.Close()
	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return []float64{}, err
	}

	ip := &ip{}
	err = json.Unmarshal(bytes, &ip)
	if err != nil {
		return []float64{}, err
	}

	locationTuple := []float64{ip.Latitude, ip.Longitude}
	fmt.Printf("found %s: %v\n", ipAddr, locationTuple)
	return locationTuple, nil
}
