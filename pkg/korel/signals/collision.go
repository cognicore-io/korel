package signals

import "math"

// Collision represents a surprising co-activation of two concepts that are
// each individually strong (high PMIMax) but rarely seen together (low joint PMI).
//
// Inspired by Daimon's "thought collisions": when two well-connected concepts
// co-activate unexpectedly, that's a learning signal. In Korel, this happens when
// a query brings together concepts the corpus hasn't connected.
type Collision struct {
	A        string  // first concept
	B        string  // second concept
	StrengthA float64 // PMIMax of A (how well-connected A is)
	StrengthB float64 // PMIMax of B (how well-connected B is)
	JointPMI float64 // actual PMI(A,B) — how often they co-occur
	Surprise float64 // expected - actual; higher = more surprising
}

// CollisionConfig controls collision detection thresholds.
type CollisionConfig struct {
	// MinStrength is the minimum PMIMax for a token to be considered "strong."
	// Only tokens above this threshold participate in collision detection.
	// Default: 0.5
	MinStrength float64

	// MaxJointPMI is the ceiling for joint PMI to qualify as a collision.
	// If PMI(A,B) exceeds this, the pair co-occurs often enough that it's
	// not surprising. Default: 0.3
	MaxJointPMI float64

	// MinSurprise filters out low-surprise collisions. Default: 0.2
	MinSurprise float64
}

// DefaultCollisionConfig returns sensible defaults.
// Thresholds use NPMI scale [-1,1] by default.
func DefaultCollisionConfig() CollisionConfig {
	return CollisionConfig{
		MinStrength: 0.3,
		MaxJointPMI: 0.15,
		MinSurprise: 0.1,
	}
}

// PMILookup provides PMI data needed for collision detection.
type PMILookup interface {
	// PMIMax returns the maximum PMI between token and any other token.
	// This measures how "well-connected" a concept is.
	PMIMax(token string) float64

	// JointPMI returns the PMI between two specific tokens.
	JointPMI(a, b string) float64
}

// DetectCollisions finds concept pairs in the query tokens that are individually
// strong but rarely seen together — the corpus "surprise signal."
//
// This is cheap to compute: we already have PMI scores. The result is a ranked
// list of collisions that could trigger expanded retrieval or rule exploration.
func DetectCollisions(tokens []string, lookup PMILookup, cfg CollisionConfig) []Collision {
	if len(tokens) < 2 || lookup == nil {
		return nil
	}

	// Compute individual strength for each token
	type tokenStrength struct {
		token    string
		strength float64
	}
	var strong []tokenStrength
	for _, tok := range tokens {
		s := lookup.PMIMax(tok)
		if s >= cfg.MinStrength {
			strong = append(strong, tokenStrength{tok, s})
		}
	}

	if len(strong) < 2 {
		return nil
	}

	// Check all pairs of strong tokens
	var collisions []Collision
	for i := 0; i < len(strong); i++ {
		for j := i + 1; j < len(strong); j++ {
			a, b := strong[i], strong[j]
			joint := lookup.JointPMI(a.token, b.token)

			if joint > cfg.MaxJointPMI {
				continue // they co-occur often, not surprising
			}

			// Expected joint PMI: geometric mean of individual strengths,
			// scaled down. Two strong concepts "should" have some relationship
			// if they're in the same domain.
			expected := math.Sqrt(a.strength*b.strength) * 0.5
			surprise := expected - joint
			if surprise < 0 {
				surprise = 0
			}

			if surprise < cfg.MinSurprise {
				continue
			}

			collisions = append(collisions, Collision{
				A:         a.token,
				B:         b.token,
				StrengthA: a.strength,
				StrengthB: b.strength,
				JointPMI:  joint,
				Surprise:  surprise,
			})
		}
	}

	// Sort by surprise descending
	sortCollisions(collisions)
	return collisions
}

func sortCollisions(c []Collision) {
	for i := 1; i < len(c); i++ {
		key := c[i]
		j := i - 1
		for j >= 0 && c[j].Surprise < key.Surprise {
			c[j+1] = c[j]
			j--
		}
		c[j+1] = key
	}
}
