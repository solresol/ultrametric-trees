package main

import (
	"database/sql"
	"encoding/csv"
	"flag"
	"fmt"
	"image/color"
	"log"
	"math/rand"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"

	"github.com/solresol/ultrametric-trees/pkg/node"
)

type AnalysisResult struct {
	Timestamp       time.Time
	SumLoss         float64
	NodeCount       int
	AvgDataQuantity float64
}

func main() {
	dbPath := flag.String("db", "", "Path to the SQLite database file")
	tableName := flag.String("table", "nodes", "Name of the nodes table")
	outputFormat := flag.String("output", "csv", "Output format: csv or png")
	flag.Parse()

	if *dbPath == "" {
		log.Fatal("Please provide a path to the database file using the -db flag")
	}

	db, err := sql.Open("sqlite3", *dbPath)
	if err != nil {
		log.Fatalf("Error opening database: %v", err)
	}
	defer db.Close()

	nodes, err := node.FetchNodes(db, *tableName)
	if err != nil {
		log.Fatalf("Error fetching nodes: %v", err)
	}

	results := analyzeNodes(nodes)

	switch *outputFormat {
	case "csv":
		outputCSV(results)
	case "png":
		outputPNG(results)
	default:
		log.Fatalf("Unsupported output format: %s", *outputFormat)
	}
}




func analyzeNodes(nodes []node.Node) []AnalysisResult {
	timestamps := node.GetSignificantTimestamps(nodes)
	var results []AnalysisResult

	for _, timestamp := range timestamps {
		relevantNodes := node.FilterNodes(nodes, timestamp, false)
		if len(relevantNodes) > 0 {
			result := calculateResult(relevantNodes, timestamp)
			results = append(results, result)
		}
	}

	return results
}

func calculateResult(nodes []node.Node, timestamp time.Time) AnalysisResult {
	var sumLoss float64
	var sumDataQuantity int64
	var validDataQuantityCount int64
	for _, n := range nodes {
		if n.Loss.Valid {
			sumLoss += n.Loss.Float64
		}
		if n.DataQuantity.Valid {
			sumDataQuantity += n.DataQuantity.Int64
			validDataQuantityCount++
		}
	}
	avgDataQuantity := float64(sumDataQuantity) / float64(validDataQuantityCount)
	if validDataQuantityCount == 0 {
		avgDataQuantity = 0
	}
	return AnalysisResult{
		Timestamp:       timestamp,
		SumLoss:         sumLoss,
		NodeCount:       len(nodes),
		AvgDataQuantity: avgDataQuantity,
	}
}



func outputCSV(results []AnalysisResult) {
	w := csv.NewWriter(os.Stdout)
	defer w.Flush()

	w.Write([]string{"Timestamp", "Sum of Losses", "Number of Nodes", "Average Data Quantity"})

	for _, r := range results {
		w.Write([]string{
			r.Timestamp.Format(time.RFC3339),
			fmt.Sprintf("%f", r.SumLoss),
			fmt.Sprintf("%d", r.NodeCount),
			fmt.Sprintf("%f", r.AvgDataQuantity),
		})
	}
}

func outputPNG(results []AnalysisResult) {
	p := plot.New()
	p.Title.Text = "Node Analysis Results"
	p.X.Label.Text = "Time"
	p.Y.Label.Text = "Values"

	sumLossData := make(plotter.XYs, len(results))
	nodeCountData := make(plotter.XYs, len(results))
	avgDataQuantityData := make(plotter.XYs, len(results))

	for i, r := range results {
		x := float64(r.Timestamp.Unix())
		sumLossData[i].X = x
		sumLossData[i].Y = r.SumLoss
		nodeCountData[i].X = x
		nodeCountData[i].Y = float64(r.NodeCount)
		avgDataQuantityData[i].X = x
		avgDataQuantityData[i].Y = r.AvgDataQuantity
	}

	addLine(p, sumLossData, "Sum of Losses")
	addLine(p, nodeCountData, "Number of Nodes")
	addLine(p, avgDataQuantityData, "Average Data Quantity")

	if err := p.Save(8*vg.Inch, 4*vg.Inch, "results.png"); err != nil {
		log.Fatal(err)
	}
}

func addLine(p *plot.Plot, data plotter.XYs, name string) {
	line, err := plotter.NewLine(data)
	if err != nil {
		log.Fatal(err)
	}
	line.Color = color.RGBA{R: uint8(rand.Intn(255)), G: uint8(rand.Intn(255)), B: uint8(rand.Intn(255)), A: 255}
	p.Add(line)
	p.Legend.Add(name, line)
}
