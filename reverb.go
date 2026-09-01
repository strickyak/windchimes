package main

type combFilter struct {
	buffer   []float32
	bufIndex int
	feedback float32
}

func newCombFilter(size int, feedback float32) combFilter {
	return combFilter{
		buffer:   make([]float32, size),
		feedback: feedback,
	}
}

func (c *combFilter) process(input float32) float32 {
	output := c.buffer[c.bufIndex]
	c.buffer[c.bufIndex] = input + output*c.feedback
	c.bufIndex++
	if c.bufIndex >= len(c.buffer) {
		c.bufIndex = 0
	}
	return output
}

type allPassFilter struct {
	buffer   []float32
	bufIndex int
	feedback float32
}

func newAllPassFilter(size int, feedback float32) allPassFilter {
	return allPassFilter{
		buffer:   make([]float32, size),
		feedback: feedback,
	}
}

func (a *allPassFilter) process(input float32) float32 {
	bufVal := a.buffer[a.bufIndex]
	output := -input + bufVal
	a.buffer[a.bufIndex] = input + bufVal*a.feedback
	a.bufIndex++
	if a.bufIndex >= len(a.buffer) {
		a.bufIndex = 0
	}
	return output
}

type StereoReverb struct {
	combL    [4]combFilter
	combR    [4]combFilter
	allPassL [2]allPassFilter
	allPassR [2]allPassFilter
}

func NewStereoReverb() *StereoReverb {
	fb := float32(0.82)
	apFb := float32(0.5)

	return &StereoReverb{
		combL: [4]combFilter{
			newCombFilter(1116, fb),
			newCombFilter(1188, fb),
			newCombFilter(1277, fb),
			newCombFilter(1356, fb),
		},
		combR: [4]combFilter{
			newCombFilter(1139, fb),
			newCombFilter(1211, fb),
			newCombFilter(1300, fb),
			newCombFilter(1379, fb),
		},
		allPassL: [2]allPassFilter{
			newAllPassFilter(225, apFb),
			newAllPassFilter(341, apFb),
		},
		allPassR: [2]allPassFilter{
			newAllPassFilter(237, apFb),
			newAllPassFilter(353, apFb),
		},
	}
}

func (r *StereoReverb) Process(inL, inR float32) (float32, float32) {
	var sumL, sumR float32
	for i := 0; i < 4; i++ {
		sumL += r.combL[i].process(inL)
		sumR += r.combR[i].process(inR)
	}

	outL := r.allPassL[0].process(sumL * 0.25)
	outL = r.allPassL[1].process(outL)

	outR := r.allPassR[0].process(sumR * 0.25)
	outR = r.allPassR[1].process(outR)

	return outL, outR
}
