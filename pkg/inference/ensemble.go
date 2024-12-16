package inference

import (
    "fmt"
    "math"
    "github.com/solresol/ultrametric-trees/pkg/exemplar"
)

// EnsemblingModel represents a collection of ModelInference structures
// that can be used together to make predictions
type EnsemblingModel struct {
    models []*ModelInference
}

// NewEnsemblingModel creates a new EnsemblingModel from an array of ModelInference pointers
func NewEnsemblingModel(models []*ModelInference) *EnsemblingModel {
    return &EnsemblingModel{
        models: models,
    }
}

// InferFromEnsemble performs inference using all models in the ensemble and
// selects the best prediction based on consensus
func (em *EnsemblingModel) InferFromEnsemble(context []string) (*InferenceResult, error) {
    if len(em.models) == 0 {
        return nil, fmt.Errorf("no models in ensemble")
    }

    // Get predictions from all models
    predictions := make([]InferenceResult, 0, len(em.models))
    synsetPaths := make([]exemplar.Synsetpath, 0, len(em.models))

    for _, model := range em.models {
        prediction, err := model.InferSingle(context)
        if err != nil {
            return nil, fmt.Errorf("error getting prediction from model: %v", err)
        }
        
        // Parse the predicted path into a Synsetpath
        synsetpath, err := exemplar.ParseSynsetpath(prediction.PredictedPath)
        if err != nil {
            return nil, fmt.Errorf("error parsing synsetpath: %v", err)
        }

        predictions = append(predictions, *prediction)
        synsetPaths = append(synsetPaths, synsetpath)
    }

    // Find the Synsetpath with lowest total cost against all others
    bestIndex := -1
    lowestTotalCost := math.MaxFloat64

    for i, candidate := range synsetPaths {
        totalCost := 0.0
        for j, other := range synsetPaths {
            if i != j {
                totalCost += exemplar.CalculateCost(candidate, other)
            }
        }

        if totalCost < lowestTotalCost {
            lowestTotalCost = totalCost
            bestIndex = i
        }
    }

    if bestIndex == -1 {
        return nil, fmt.Errorf("could not find best prediction from ensemble")
    }

    // Return the InferenceResult corresponding to the best Synsetpath
    return &predictions[bestIndex], nil
}
