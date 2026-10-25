package model

import (
	"math"
	"time"
)

// Port of web/packages/core/src/spring-layout.ts. Products that feed a sum are wrapped in float64(...):
// without it the Go compiler may fuse them into an FMA on arm64, which breaks bit parity with Swift/TS.

// MakeDrand48 is POSIX drand48 with the default seed (0x1234abcd330e), which is what Swift's unseeded
// drand48() yields in a fresh process. Pass a seed to start elsewhere.
func MakeDrand48(seed uint64) func() float64 {
	x := seed
	return func() float64 {
		x = (0x5deece66d*x + 0xb) & 0xffffffffffff
		return float64(x) / (1 << 48)
	}
}

// DefaultDrand48Seed is the seed of an unseeded drand48.
const DefaultDrand48Seed uint64 = 0x1234abcd330e

// Defaults of the spring parameters, and the minimum distance considered between nodes.
const (
	defaultSpringIterations  = 1000
	maxSpringTime            = 1 // seconds the algorithm runs
	defaultSpringMove        = 1.0
	defaultSpringStrain      = 1.0
	defaultSpringLength      = 3.0
	defaultSpringGravitation = 2
	minDistance              = 50.0
)

type SpringLayout struct {
	SprIterations int
	// MaxTimeMS: Swift divides elapsed seconds by this, so despite the name it is seconds.
	MaxTimeMS      float64
	SprRandom      bool
	SprMove        float64
	SprStrain      float64
	SprLength      float64
	SprGravitation float64

	// Iteration is a float64 because the time based jump may set it beyond any Int.
	Iteration             float64
	SrcDestToSumOfWeights [][]float64
	Entities              []*DiagramItem
	ForcesX               []float64
	ForcesY               []float64
	LocationsX            []float64
	LocationsY            []float64
	SizeW                 []float64
	SizeH                 []float64
	Bounds                Rect
	BoundsScaleX          float64
	BoundsScaleY          float64

	LayoutContext *LayoutContext

	// FitWithinBoundsEnabled is the Swift `fitWithinBounds` var, which shares its name with the method.
	FitWithinBoundsEnabled bool

	Operations []ElementOperation

	StartTime float64

	// Now (seconds) and Random are injectable so runs are reproducible.
	Now    func() float64
	Random func() float64
}

// NewSpringLayout takes a seconds clock and a random source; nil means wall time and the default drand48.
func NewSpringLayout(now, random func() float64) *SpringLayout {
	if now == nil {
		now = func() float64 { return float64(time.Now().UnixMilli()) / 1000 }
	}
	if random == nil {
		random = MakeDrand48(DefaultDrand48Seed)
	}
	return &SpringLayout{
		SprIterations:  defaultSpringIterations,
		MaxTimeMS:      maxSpringTime,
		SprMove:        defaultSpringMove,
		SprStrain:      defaultSpringStrain,
		SprLength:      defaultSpringLength,
		SprGravitation: defaultSpringGravitation,

		BoundsScaleX:           1,
		BoundsScaleY:           1,
		FitWithinBoundsEnabled: true,
		Now:                    now,
		Random:                 random,
	}
}

func (s *SpringLayout) Apply(context *LayoutContext, clean bool) ([]ElementOperation, error) {
	s.LayoutContext = context
	s.initLayout(context)
	if !clean {
		return nil, nil
	}

	for s.PerformAnotherNonContinuousIteration() {
		s.ComputeOneIteration()
	}

	s.saveLocations()

	if s.FitWithinBoundsEnabled {
		const insets = 4
		bounds2 := Rect{
			X:      s.Bounds.X + insets,
			Y:      s.Bounds.Y + insets,
			Width:  s.Bounds.Width - 2*insets,
			Height: s.Bounds.Height - 2*insets,
		}
		s.FitWithinBounds(bounds2)
		s.saveLocations()
	}
	return s.Operations, nil
}

func (s *SpringLayout) FitWithinBounds(destinationBounds Rect) {
	if len(s.Entities) == 1 {
		return
	}
	startingBounds := s.getLayoutBounds()
	for i, entity := range s.Entities {
		size := s.LayoutContext.GetNodeBounds(entity)
		if s.LayoutContext.IsMovable(entity) {
			locationX := s.LocationsX[i]
			locationY := s.LocationsY[i]
			var percentX, percentY float64
			if startingBounds.Width != 0 {
				percentX = (locationX - startingBounds.X) / startingBounds.Width
			}
			if startingBounds.Height != 0 {
				percentY = (locationY - startingBounds.Y) / startingBounds.Height
			}
			s.LocationsX[i] = destinationBounds.X + size.Width/2 + float64(percentX*(destinationBounds.Width-size.Width))
			s.LocationsY[i] = destinationBounds.Y + size.Height/2 + float64(percentY*(destinationBounds.Height-size.Height))
		}
	}
}

func (s *SpringLayout) initLayout(context *LayoutContext) {
	s.Entities = context.Nodes
	s.Bounds = context.GetViewBounds()
	s.loadLocations()

	n := len(s.Entities)
	s.SrcDestToSumOfWeights = make([][]float64, n)
	for i := range s.SrcDestToSumOfWeights {
		s.SrcDestToSumOfWeights[i] = make([]float64, n)
	}
	entityToPosition := make(map[*DiagramItem]int, n)
	for i, e := range s.Entities {
		entityToPosition[e] = i
	}

	for _, connection := range context.Edges {
		if connection.Source == nil || connection.Target == nil {
			continue
		}
		source, okS := entityToPosition[connection.Source]
		target, okT := entityToPosition[connection.Target]
		if okS && okT {
			weight := context.GetWeight(connection)
			if weight <= 0 {
				weight = 0.1
			}
			s.SrcDestToSumOfWeights[source][target] += weight
			s.SrcDestToSumOfWeights[target][source] += weight
		}
	}

	if s.SprRandom {
		s.PlaceRandomly() // put vertices in random places
	}

	s.Iteration = 1

	s.StartTime = s.Now()
}

func (s *SpringLayout) loadLocations() {
	if len(s.LocationsX) != len(s.Entities) {
		length := len(s.Entities)
		s.LocationsX = make([]float64, length)
		s.LocationsY = make([]float64, length)
		s.SizeW = make([]float64, length)
		s.SizeH = make([]float64, length)
		s.ForcesX = make([]float64, length)
		s.ForcesY = make([]float64, length)
	}
	for i, ent := range s.Entities {
		s.LocationsX[i] = ent.X
		s.LocationsY[i] = ent.Y
		size := s.LayoutContext.GetNodeBounds(ent)
		s.SizeW[i] = size.Width
		s.SizeH[i] = size.Height
	}
}

func (s *SpringLayout) saveLocations() {
	if len(s.Entities) == 0 {
		return
	}
	s.Operations = nil
	for i, e := range s.Entities {
		// NaN positions are reset to the origin (Swift TODO: find where they come from).
		if math.IsNaN(s.LocationsX[i]) || math.IsNaN(s.LocationsY[i]) {
			s.LocationsX[i] = 0
			s.LocationsY[i] = 0
		}
		s.Operations = append(s.Operations, s.LayoutContext.Store.CreateUpdatePosition(e, Point{s.LocationsX[i], s.LocationsY[i]}))
	}
}

// setSprIterationsBasedOnTime scales the iteration counter by wall time: after MaxTimeMS (seconds)
// the algorithm jumps to the last iteration.
func (s *SpringLayout) setSprIterationsBasedOnTime() {
	if s.MaxTimeMS <= 0 {
		return
	}

	since := s.Now() - s.StartTime
	fractionComplete := since / s.MaxTimeMS
	currentIteration := math.Trunc(fractionComplete * float64(s.SprIterations))
	if currentIteration > s.Iteration {
		s.Iteration = currentIteration
	}
}

func (s *SpringLayout) PerformAnotherNonContinuousIteration() bool {
	s.setSprIterationsBasedOnTime()
	return s.Iteration <= float64(s.SprIterations)
}

func (s *SpringLayout) ComputeOneIteration() {
	s.ComputeForces()
	s.ComputePositions()
	currentBounds := s.getLayoutBounds()
	s.improveBoundScaleX(currentBounds)
	s.improveBoundScaleY(currentBounds)
	s.moveToCenter(currentBounds)
	s.Iteration++
}

// PlaceRandomly puts vertices in random places inside the view bounds.
func (s *SpringLayout) PlaceRandomly() {
	if len(s.LocationsX) == 0 {
		return
	}

	// If only one node in the data repository, put it in the middle
	if len(s.LocationsX) == 1 {
		s.LocationsX[0] = s.Bounds.X + float64(0.5*s.Bounds.Width)
		s.LocationsY[0] = s.Bounds.Y + float64(0.5*s.Bounds.Height)
		return
	}
	s.LocationsX[0] = s.Bounds.X
	s.LocationsY[0] = s.Bounds.Y
	s.LocationsX[1] = s.Bounds.X + s.Bounds.Width
	s.LocationsY[1] = s.Bounds.Y + s.Bounds.Height
	for i := 2; i < len(s.LocationsX); i++ {
		s.LocationsX[i] = s.Bounds.X + float64(s.Random()*s.Bounds.Width)
		s.LocationsY[i] = s.Bounds.Y + float64(s.Random()*s.Bounds.Height)
	}
}

// ComputeForces computes the force for each node. Two passes: the second starts from positions moved by the first.
func (s *SpringLayout) ComputeForces() {
	count := len(s.ForcesX)
	forcesX := [2][]float64{make([]float64, count), make([]float64, count)}
	forcesY := [2][]float64{make([]float64, count), make([]float64, count)}
	locationsX := make([]float64, count)
	locationsY := make([]float64, len(s.ForcesY))

	for i := 0; i < count; i++ {
		locationsX[i] = s.LocationsX[i]
		locationsY[i] = s.LocationsY[i]
	}

	for k := 0; k < 2; k++ {
		fx := forcesX[k]
		fy := forcesY[k]
		for i := 0; i < len(s.LocationsX); i++ {
			for j := i + 1; j < len(s.LocationsX); j++ {
				// Avoid division by zero when bounds width/height are zero.
				bw := swiftMax(s.Bounds.Width, 1e-6)
				bh := swiftMax(s.Bounds.Height, 1e-6)
				dx := (locationsX[i] - locationsX[j]) / bw / s.BoundsScaleX
				dy := (locationsY[i] - locationsY[j]) / bh / s.BoundsScaleY
				distanceSq := float64(dx*dx) + float64(dy*dy)
				// make sure distance and distance squared not too small
				distanceSq = swiftMax(float64(minDistance*minDistance), distanceSq)
				distance := math.Sqrt(distanceSq)

				// If there are relationships between srcObj and destObj then decrease force on srcObj (a pull) in direction of destObj.
				// If no relation between srcObj and destObj then increase force on srcObj (a push) from direction of destObj.
				sumOfWeights := s.SrcDestToSumOfWeights[i][j]

				var f float64
				if sumOfWeights > 0 {
					// nodes are pulled towards each other
					f = -s.SprStrain * math.Log(distance/s.SprLength) * sumOfWeights
				} else {
					// nodes are repelled from each other
					f = s.SprGravitation / distanceSq
				}
				dfx := (f * dx) / distance
				dfy := (f * dy) / distance

				fx[i] += dfx
				fy[i] += dfy

				fx[j] -= dfx
				fy[j] -= dfy
			}
		}

		for i, e := range s.Entities {
			if s.LayoutContext.IsMovable(e) {
				deltaX := s.SprMove * fx[i]
				deltaY := s.SprMove * fy[i]

				// constrain movement, so that nodes don't shoot way off to the edge
				dist := math.Sqrt(float64(deltaX*deltaX) + float64(deltaY*deltaY))
				maxMovement := 0.2 * s.SprMove
				if dist > maxMovement {
					deltaX = (deltaX * maxMovement) / dist
					deltaY = (deltaY * maxMovement) / dist
				}

				locationsX[i] += float64(deltaX * s.Bounds.Width * s.BoundsScaleX)
				locationsY[i] += float64(deltaY * s.Bounds.Height * s.BoundsScaleY)
			}
		}
	}
	for i := range s.Entities {
		if float64(forcesX[0][i]*forcesX[1][i]) < 0 {
			s.ForcesX[i] = 0
		} else {
			s.ForcesX[i] = forcesX[1][i]
		}
		if float64(forcesY[0][i]*forcesY[1][i]) < 0 {
			s.ForcesY[i] = 0
		} else {
			s.ForcesY[i] = forcesY[1][i]
		}
	}
}

// ComputePositions: position = position + sprMove * force.
func (s *SpringLayout) ComputePositions() {
	for i, e := range s.Entities {
		if s.LayoutContext.IsMovable(e) {
			deltaX := s.SprMove * s.ForcesX[i]
			deltaY := s.SprMove * s.ForcesY[i]

			// constrain movement, so that nodes don't shoot way off to the edge
			dist := math.Sqrt(float64(deltaX*deltaX) + float64(deltaY*deltaY))
			maxMovement := 0.2 * s.SprMove
			if dist > maxMovement {
				deltaX = (deltaX * maxMovement) / dist
				deltaY = (deltaY * maxMovement) / dist
			}

			s.LocationsX[i] += float64(deltaX * s.Bounds.Width * s.BoundsScaleX)
			s.LocationsY[i] += float64(deltaY * s.Bounds.Height * s.BoundsScaleY)
		}
	}
}

func (s *SpringLayout) getLayoutBounds() Rect {
	minX := math.Inf(1)
	minY := math.Inf(1)
	maxX := math.Inf(-1)
	maxY := math.Inf(-1)

	for i := range s.LocationsX {
		maxX = swiftMax(maxX, s.LocationsX[i]+s.SizeW[i])
		minX = swiftMin(minX, s.LocationsX[i]-s.SizeW[i])
		maxY = swiftMax(maxY, s.LocationsY[i]+s.SizeH[i])
		minY = swiftMin(minY, s.LocationsY[i]-s.SizeH[i])
	}
	return Rect{X: minX, Y: minY, Width: maxX - minX, Height: maxY - minY}
}

func (s *SpringLayout) improveBoundScaleX(currentBounds Rect) {
	boundaryProportionX := currentBounds.Width / s.Bounds.Width
	if boundaryProportionX < 0.9 {
		s.BoundsScaleX *= 1.01
	} else if boundaryProportionX > 1 {
		if s.BoundsScaleX < 0.01 {
			return
		}
		s.BoundsScaleX /= 1.01
	}
}

func (s *SpringLayout) improveBoundScaleY(currentBounds Rect) {
	boundaryProportionY := currentBounds.Height / s.Bounds.Height
	if boundaryProportionY < 0.9 {
		s.BoundsScaleY *= 1.01
	} else if boundaryProportionY > 1 {
		if s.BoundsScaleY < 0.01 {
			return
		}
		s.BoundsScaleY /= 1.01
	}
}

func (s *SpringLayout) moveToCenter(currentBounds Rect) {
	moveX := currentBounds.X + currentBounds.Width/2 - (s.Bounds.X + s.Bounds.Width/2)
	moveY := currentBounds.Y + currentBounds.Height/2 - (s.Bounds.Y + s.Bounds.Height/2)
	for i := range s.LocationsX {
		s.LocationsX[i] -= moveX
		s.LocationsY[i] -= moveY
	}
}
