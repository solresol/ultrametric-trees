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
	TrainingDataSize int
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

	results, err := analyzeNodes(db, *tableName, nodes)
	if err != nil {
		log.Fatalf("Could not analyse nodes: %v", err)
	}

	switch *outputFormat {
	case "csv":
		outputCSV(results)
	case "png":
		outputPNG(results)
	default:
		log.Fatalf("Unsupported output format: %s", *outputFormat)
	}
}

func analyzeNodes(db *sql.DB, tableName string, nodes []node.Node) ([]AnalysisResult, error) {
	timestamps := node.GetSignificantTimestamps(nodes)
	var results []AnalysisResult

	for _, timestamp := range timestamps {
		relevantNodes, err := node.FetchNodesAsOf(db, tableName, timestamp)
		if err != nil {
			return results, err
		}
		if len(relevantNodes) > 0 {
			result := calculateResult(relevantNodes, timestamp)
			results = append(results, result)
		}
	}

	return results, nil
}

func calculateResult(nodes []node.Node, timestamp time.Time) AnalysisResult {
	sumLoss := 0.0
	sumDataQuantity := 0
	leafNodeCount := 0
	for _, n := range nodes {
		if n.HasChildren {
			continue
		}
		if n.Loss.Valid {
			sumLoss += n.Loss.Float64
		}
		// 
		if n.DataQuantity.Valid {
			sumDataQuantity += int(n.DataQuantity.Int64)
			leafNodeCount++
		}
	}
	avgDataQuantity := float64(sumDataQuantity) / float64(leafNodeCount)
	return AnalysisResult{
		Timestamp:       timestamp,
		SumLoss:         sumLoss,
		NodeCount:       len(nodes),
		AvgDataQuantity: avgDataQuantity,
		TrainingDataSize: sumDataQuantity,
	}
}

func outputCSV(results []AnalysisResult) {
	w := csv.NewWriter(os.Stdout)
	defer w.Flush()

	w.Write([]string{"Timestamp", "Sum of Losses", "Number of Nodes", "Average Data Quantity", "Training Data Size"})

	for _, r := range results {
		w.Write([]string{
			r.Timestamp.Format(time.RFC3339),
			fmt.Sprintf("%f", r.SumLoss),
			fmt.Sprintf("%d", r.NodeCount),
			fmt.Sprintf("%f", r.AvgDataQuantity),
			fmt.Sprintf("%d", r.TrainingDataSize),
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
