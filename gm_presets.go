package main

type GMPreset struct {
	Name         string
	Harmonics    []float64
	Attack       float64
	Decay        float64
	Sustain      float64
	Release      float64
	Gain         float32
	Chorus       bool
	VibratoRate  float64
	VibratoDepth float64
}

// GetGMPreset returns acoustic instrument synthesis parameters for a given General MIDI program (0-127).
func GetGMPreset(program uint8) GMPreset {
	switch {
	// --- Pianos (0 - 5) ---
	case program <= 5:
		return GMPreset{
			Name:      "Acoustic Piano",
			Harmonics: []float64{1.0, 0.6, 0.35, 0.2, 0.1, 0.05, 0.02},
			Attack:    0.003,
			Decay:     1.5,
			Sustain:   0.05,
			Release:   0.4,
			Gain:      1.0,
		}

	// --- Harpsichord & Clavinet (6 - 7) ---
	case program == 6 || program == 7:
		return GMPreset{
			Name:      "Harpsichord",
			Harmonics: []float64{1.0, 0.8, 0.65, 0.5, 0.4, 0.3, 0.2, 0.15, 0.1},
			Attack:    0.001,
			Decay:     0.8,
			Sustain:   0.0,
			Release:   0.15,
			Gain:      0.85,
		}

	// --- Chromatic Percussion: Celesta, Glockenspiel, Music Box, Bells (8 - 15) ---
	case program >= 8 && program <= 15:
		return GMPreset{
			Name:      "Bells/Celesta",
			Harmonics: []float64{1.0, 0.15, 0.6, 0.08, 0.35, 0.04, 0.2},
			Attack:    0.001,
			Decay:     1.2,
			Sustain:   0.0,
			Release:   0.6,
			Gain:      0.9,
		}

	// --- Church Organ (19) ---
	case program == 19:
		return GMPreset{
			Name:      "Church Organ",
			Harmonics: []float64{1.0, 0.8, 0.6, 0.5, 0.3, 0.4, 0.2, 0.1},
			Attack:    0.015,
			Decay:     0.05,
			Sustain:   0.95,
			Release:   0.25,
			Gain:      0.75,
		}

	// --- Other Organs & Accordions (16 - 23) ---
	case program >= 16 && program <= 23:
		return GMPreset{
			Name:      "Organ",
			Harmonics: []float64{1.0, 0.7, 0.5, 0.4, 0.2, 0.3, 0.15},
			Attack:    0.01,
			Decay:     0.05,
			Sustain:   0.9,
			Release:   0.2,
			Gain:      0.8,
		}

	// --- Guitars (24 - 31) ---
	case program >= 24 && program <= 31:
		return GMPreset{
			Name:      "Acoustic Guitar",
			Harmonics: []float64{1.0, 0.7, 0.4, 0.25, 0.15, 0.08, 0.04},
			Attack:    0.002,
			Decay:     1.0,
			Sustain:   0.03,
			Release:   0.25,
			Gain:      0.95,
		}

	// --- Basses (32 - 39) ---
	case program >= 32 && program <= 39:
		return GMPreset{
			Name:      "Acoustic Bass",
			Harmonics: []float64{1.0, 0.85, 0.5, 0.3, 0.15, 0.05},
			Attack:    0.008,
			Decay:     0.4,
			Sustain:   0.65,
			Release:   0.3,
			Gain:      1.2, // Boost low bass clarity
		}

	// --- Solo Violin / Viola / Cello / Contrabass (40 - 43) ---
	case program >= 40 && program <= 43:
		return GMPreset{
			Name:         "Solo Strings",
			Harmonics:    []float64{1.0, 0.75, 0.55, 0.42, 0.32, 0.24, 0.18, 0.14, 0.1, 0.07},
			Attack:       0.03,
			Decay:        0.08,
			Sustain:      0.85,
			Release:      0.35,
			Gain:         0.9,
			VibratoRate:  5.5,
			VibratoDepth: 0.0035,
		}

	// --- Pizzicato Strings (45) ---
	case program == 45:
		return GMPreset{
			Name:      "Pizzicato Strings",
			Harmonics: []float64{1.0, 0.6, 0.3, 0.15, 0.05},
			Attack:    0.001,
			Decay:     0.45,
			Sustain:   0.0,
			Release:   0.15,
			Gain:      1.0,
		}

	// --- Orchestral Harp (46) ---
	case program == 46:
		return GMPreset{
			Name:      "Orchestral Harp",
			Harmonics: []float64{1.0, 0.5, 0.25, 0.12, 0.06},
			Attack:    0.002,
			Decay:     1.8,
			Sustain:   0.0,
			Release:   0.5,
			Gain:      0.9,
		}

	// --- String Ensembles & Synth Strings (48 - 51) ---
	case program >= 48 && program <= 51:
		return GMPreset{
			Name:      "String Ensemble",
			Harmonics: []float64{1.0, 0.75, 0.58, 0.45, 0.35, 0.26, 0.2, 0.15, 0.1, 0.08, 0.05},
			Attack:    0.04,
			Decay:     0.1,
			Sustain:   0.88,
			Release:   0.45,
			Gain:      0.75,
			Chorus:    true, // Dual-phase ensemble detuning
		}

	// --- Choirs & Voices (52 - 55) ---
	case program >= 52 && program <= 55:
		return GMPreset{
			Name:      "Choir",
			Harmonics: []float64{1.0, 0.25, 0.7, 0.15, 0.4, 0.08, 0.25},
			Attack:    0.08,
			Decay:     0.15,
			Sustain:   0.9,
			Release:   0.5,
			Gain:      0.75,
			Chorus:    true,
		}

	// --- Trumpet & Brass (56 - 63) ---
	case program >= 56 && program <= 63:
		return GMPreset{
			Name:      "Brass",
			Harmonics: []float64{1.0, 0.9, 0.75, 0.6, 0.45, 0.3, 0.2, 0.1},
			Attack:    0.02,
			Decay:     0.08,
			Sustain:   0.8,
			Release:   0.25,
			Gain:      0.85,
		}

	// --- Oboe, English Horn, Clarinet, Reeds (64 - 71) ---
	case program >= 64 && program <= 71:
		return GMPreset{
			Name:         "Reeds/Oboe",
			Harmonics:    []float64{0.7, 1.0, 0.8, 0.65, 0.45, 0.3, 0.18, 0.1},
			Attack:       0.025,
			Decay:        0.06,
			Sustain:      0.85,
			Release:      0.2,
			Gain:         0.85,
			VibratoRate:  5.0,
			VibratoDepth: 0.0025,
		}

	// --- Flutes & Pipes (72 - 79) ---
	case program >= 72 && program <= 79:
		return GMPreset{
			Name:         "Flute/Pipes",
			Harmonics:    []float64{1.0, 0.2, 0.08, 0.03},
			Attack:       0.035,
			Decay:        0.05,
			Sustain:      0.9,
			Release:      0.25,
			Gain:         0.9,
			VibratoRate:  5.2,
			VibratoDepth: 0.003,
		}

	// --- Synth Leads & Pads (80 - 103) ---
	case program >= 80 && program <= 103:
		return GMPreset{
			Name:      "Synth Pad",
			Harmonics: []float64{1.0, 0.7, 0.5, 0.35, 0.25, 0.18, 0.12},
			Attack:    0.05,
			Decay:     0.1,
			Sustain:   0.85,
			Release:   0.4,
			Gain:      0.75,
			Chorus:    true,
		}

	// --- Default Fallback ---
	default:
		return GMPreset{
			Name:      "Default Harmonic",
			Harmonics: []float64{1.0, 0.5, 0.25, 0.12},
			Attack:    0.01,
			Decay:     0.1,
			Sustain:   0.8,
			Release:   0.3,
			Gain:      0.8,
		}
	}
}
