// Command taxparcels is a utility for Pierce County's Tax Parcel
// GIS data.
package main

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	geojson "github.com/paulmach/go.geojson"
	"golang.org/x/exp/slices"
)

//go:generate go run github.com/miku/zek/cmd/zek -B -P main -C -m -o zkml.go tax_parcels.kml

func main() {
	if err := main1(); err != nil {
		panic(err)
	}
}

func main1() error {
	var (
		idPaths  string
		kmlPath  string
		jsonPath string
		c        config
	)
	flag.StringVar(&idPaths, "ids", "", "comma-delimited list of paths of parcel IDs file")
	flag.StringVar(&kmlPath, "kml", "", "path to the input KML file")
	flag.StringVar(&jsonPath, "json", "", "path to the input GeoJSON file")
	flag.StringVar(&c.outDir, "out", "", "path to the directory where the output will be written")
	flag.Parse()

	paths := strings.Split(idPaths, ",")
	if jsonPath != "" {
		return c.filterJSON(jsonPath, paths...)
	}
	if kmlPath != "" {
		return c.filterKML(kmlPath, paths...)
	}
	return errors.New("must supply -json or -kml")
}

type config struct {
	outDir string
}

func (c config) filterJSON(inputPath string, idPaths ...string) error {
	fc, err := parseJSON(inputPath)
	if err != nil {
		return fmt.Errorf("unable to parse GeoJSON: %w", err)
	}

	for _, path := range idPaths {
		ids, err := parseIDs(path)
		if err != nil {
			return fmt.Errorf("unable to parse parcel ID file: %w", err)
		}
		fmt.Fprintf(os.Stderr, "parsed %d from %q\n", len(ids), path)

		out := fc
		out.Features = nil

		for _, f := range fc.Features {
			tpn, ok := f.Properties["TaxParcelNumber"].(string)
			if ok && ids[tpn] {
				delete(ids, tpn)
				out.Features = append(out.Features, f)
				continue
			}
		}

		fmt.Fprintf(os.Stderr, "found %d of %d\n",
			len(out.Features), len(out.Features)+len(ids))
		for id := range ids {
			fmt.Fprintf(os.Stderr, "could not find: %s\n", id)
		}

		path = strings.TrimSuffix(
			filepath.Base(path), filepath.Ext(path)) + ".geojson"
		f, err := os.Create(filepath.Join(c.outDir, path))
		if err != nil {
			return fmt.Errorf("unable to create file: %w", err)
		}
		defer f.Close()

		if err := json.NewEncoder(f).Encode(out); err != nil {
			return fmt.Errorf("unable to encode GeoJSON: %w", err)
		}
		if err := f.Close(); err != nil {
			return fmt.Errorf("unable to close file: %w", err)
		}
	}
	return nil
}

func (c config) filterKML(inputPath string, idPaths ...string) error {
	v, err := parseKML(inputPath)
	if err != nil {
		return fmt.Errorf("unable to parse XML file: %w", err)
	}

	marks := slices.Clone(v.Document.Folder.Placemark)
	for _, path := range idPaths {
		ids, err := parseIDs(path)
		if err != nil {
			return fmt.Errorf("unable to parse parcel ID file: %w", err)
		}
		fmt.Fprintf(os.Stderr, "parsed %d from %q\n", len(ids), path)

		found := v.Document.Folder.Placemark[:0]
		for _, p := range marks {
		Loop:
			for _, d := range p.ExtendedData.SchemaData.SimpleData {
				if d.Name == "TaxParcelNumber" && ids[d.Text] {
					delete(ids, d.Text)
					found = append(found, p)
					break Loop
				}
			}
		}
		v.Document.Folder.Placemark = found

		fmt.Fprintf(os.Stderr, "found %d of %d\n", len(found), len(found)+len(ids))
		for id := range ids {
			fmt.Fprintf(os.Stderr, "could not find: %s\n", id)
		}

		path = strings.TrimSuffix(
			filepath.Base(path), filepath.Ext(path)) + ".kml"
		f, err := os.Create(filepath.Join(c.outDir, path))
		if err != nil {
			return fmt.Errorf("unable to create file: %w", err)
		}
		defer f.Close()

		if err := xml.NewEncoder(f).Encode(v); err != nil {
			return fmt.Errorf("unable to encode KML: %w", err)
		}
		if err := f.Close(); err != nil {
			return fmt.Errorf("unable to close file: %w", err)
		}
	}
	return nil
}

func parseJSON(path string) (geojson.FeatureCollection, error) {
	f, err := os.Open(path)
	if err != nil {
		return geojson.FeatureCollection{}, err
	}
	defer f.Close()

	var v geojson.FeatureCollection
	err = json.NewDecoder(f).Decode(&v)
	if err != nil {
		return geojson.FeatureCollection{}, err
	}
	return v, nil
}

func parseKML(path string) (Kml, error) {
	in, err := os.Open(path)
	if err != nil {
		return Kml{}, err
	}
	defer in.Close()

	var v Kml
	err = xml.NewDecoder(in).Decode(&v)
	if err != nil {
		return Kml{}, err
	}
	return v, nil
}

func parseIDs(path string) (map[string]bool, error) {
	buf, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	m := make(map[string]bool)
	for _, s := range strings.Split(string(buf), "\n") {
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		m[s] = true
	}
	return m, nil
}
