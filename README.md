# Windchimes.

This program (when in -mode=wind3) plays random stochastic tones in stero
with lots of controls for how it is done.

The output of the program is headerless "PWM Format" with 2 channels of
Signed 16 bit Little Endian integer values.

You can specify the octaves, the scales, the envelope,
the harmonics of the fundamental tone, and the frequency of poisson arrival.

```text
Usage of /tmp/go-build3761695214/b001/exe/windchime:
  -a float
    	Attack time in seconds (default 0.1)
  -d float
    	Decay time in seconds (default 0.2)
  -even string
    	Even-tempered scale notes (e.g. A,C,D,E,G or C,E,G,Bb) (default "A,C,D,E,G")
  -h string
    	Comma-separated float64 harmonic vector (default "8,4,2,1")
  -just string
    	Just tuning ratios, e.g. '1/1,6/5,4/3,3/2,9/5' or 'pentatonic'
  -mode string
    	Wave mode: 'sine', 'harm', or 'wind3' (default "harm")
  -octaves string
    	Comma-separated list of octaves to use in wind3 mode (default "3,4,5")
  -og float
    	Output gain multiplier (default 0.3)
  -p string
    	Arrival time mean and standard deviation (m,d). If d is omitted, defaults to m/3. (default "3")
  -r float
    	Release time in seconds (default 0.5)
  -s float
    	Sustain level (proportion 0.0 to 1.0) (default 0.5)
  -t float
    	Total time for attack, decay, and sustain in seconds (default 1.3)
  -tuning float
    	Frequency of A4 in Hz (default 440)
```

-just=pentatonic means -just=1/1,6/5,4/3,3/2,9/5

```sh
  go run . -mode sine -a 0.1 -d 0.2 -s 0.5 -r 0.5 | aplay -f S16_LE -c 2 -r 48000
  go run . -mode harm -a 0.1 -d 0.2 -s 0.5 -r 0.5 | aplay -f S16_LE -c 2 -r 48000
  go run . -mode harm -h "10,4,6,3,5,2" |  aplay -f S16_LE -c 2 -r 48000
  go run . -mode harm  |  aplay -f S16_LE -c 2 -r 48000
  go run . -mode wind3 | aplay -f S16_LE -c 2 -r 48000
  go run . -mode wind3 -a 2.0 -r 2.0 -h "10,2,8,2" | aplay -f S16_LE -c 2 -r 48000
  go run . -mode wind3 -t 3.0 | aplay -f S16_LE -c 2 -r 48000
  go run . -mode wind3 -t 3.0 -d 5 | aplay -f S16_LE -c 2 -r 48000
  go run . -mode wind3 -t 3.0 -d 5  -h=100,1,80,1,60,1,40,1,30,1,20,1 | aplay -f S16_LE -c 2 -r 48000
  go run . -mode wind3 -t 3.0 -d 5  -h=100,1,80,1,60,1,40,1,30,1,20,1 -tuning=220 | aplay -f S16_LE -c 2 -r 48000
  go run . -mode wind3   -h=100,1,80,1,60,1,40,1,30,1,20,1 -a=0.3 -d=0.2 -s=0.2 -r=10 -t=2  -tuning=220 | aplay -f S16_LE -c 2 -r 48000
  go run . -mode wind3   -h=100,1,80,1,60,1,40,1,30,1,20,1 -a=0.5 -d=0.2 -s=0.2 -r=10 -t=2  -tuning=220 | aplay -f S16_LE -c 2 -r 48000
  go run . -mode wind3   -h=100,1,80,1,60,1,40,1,30,1,20,1 -a=0.3 -d=0.3 -s=0.4 -r=10 -t=2  -tuning=220 | aplay -f S16_LE -c 2 -r 48000
  go run . -mode wind3   -h=100,1,80,1,60,1,40,1,30,1,20,1 -a=0.1 -d=0.3 -s=0.4 -r=10 -t=2  -tuning=220 | aplay -f S16_LE -c 2 -r 48000
  go run . -mode wind3   -h=100,1,80,1,60,1,40,1,30,1,20,1 -a=0.1 -d=0.3 -s=0.4 -r=10 -t=2  -tuning=110  | aplay -f S16_LE -c 2 -r 48000
  go run . -mode wind3   -h=100,1,80,1,60,1,40,1,30,1,20,1 -a=0.1 -d=0.3 -s=0.4 -r=10 -t=2  -tuning=110  | aplay -f S16_LE -c 2 -r 48000
  go run . -mode wind3   -h=100,1,80,1,60,1,40,1,30,1,20,1 -a=0.1 -d=0.3 -s=0.4 -r=10 -t=2  -octaves=1,5,9  | aplay -f S16_LE -c 2 -r 48000
  go run . -mode wind3   -h=100,1,80,1,60,1,40,1,30,1,20,1 -a=0.1 -d=0.3 -s=0.4 -r=10 -t=2  -octaves=3,3.5 | aplay -f S16_LE -c 2 -r 48000
  go run . -mode wind3   -h=100,1,80,1,60,1,40,1,30,1,20,1 -a=0.1 -d=0.3 -s=0.4 -r=10 -t=2  -octaves=3,3.5 -p=1 | aplay -f S16_LE -c 2 -r 48000
  go run . -mode wind3   -h=100,1,80,1,60,1,40,1,30,1,20,1 -a=0.1 -d=0.3 -s=0.4 -r=10 -t=2  -octaves=1,2.5,3,3.5 -p=1 | aplay -f S16_LE -c 2 -r 48000
  go run . -mode wind3   -h=100,1,80,1,60,1,40,1,30,1,20,1 -a=0.1 -d=0.3 -s=0.4 -r=10 -t=2  -octaves=1,2.5,3,3.5 -p=1 | aplay -f S16_LE -c 2 -r 48000
  go run . -mode wind3 -octaves 3,4,5 -just "1/1,5/4,3/2" | aplay -f S16_LE -c 2 -r 48000
  go run . -mode wind3 -octaves 3,4,5 -just "pentatonic" | aplay -f S16_LE -c 2 -r 48000
  go run . -mode wind3 -octaves 1,2,3,4,5 -just "pentatonic" | aplay -f S16_LE -c 2 -r 48000
  go run . -mode wind3 -octaves 1,2,3,4.7,5.5 -just "pentatonic" | aplay -f S16_LE -c 2 -r 48000
  go run . -mode wind3 -octaves 1,2,3,4.7,5.5 -just "pentatonic"  -a=0.1 -d=0.3 -s=0.4 -r=10 -t=2  -p=1  | aplay -f S16_LE -c 2 -r 48000
  go run . -mode wind3 -octaves 3 -just "1/1,3/2,4/3,5/4,6/5"  -a=0.1 -d=0.3 -s=0.4 -r=10 -t=2  -p=1  | aplay -f S16_LE -c 2 -r 48000
  go run . -mode wind3 -octaves 1,2,3,5.5 -just pentatonic -a=0.1 -d=0.3 -s=0.4 -r=10 -t=2  -p=1  | aplay -f S16_LE -c 2 -r 48000
  go run . -mode wind3 -octaves 1,2,3,4 -just 4/8,5/8,6/8,7/8 -a=0.1 -d=0.3 -s=0.4 -r=10 -t=2  -p=1  | aplay -f S16_LE -c 2 -r 48000
  go run . -mode wind3 -octaves "3+7/12, 4+7/12" | aplay -f S16_LE -c 2 -r 48000
  go run . -mode wind3 -octaves 3,4+7/12 -just pentatonic -a=0.1 -d=0.3 -s=0.4 -r=10 -t=2  -p=1  | aplay -f S16_LE -c 2 -r 48000
  go run . -mode wind3 -octaves 3,4+7/12 -just pentatonic -a=0.1 -d=0.3 -s=0.4 -r=10 -t=2  -p=1 -tuning=110 | aplay -f S16_LE -c 2 -r 48000
  go run . --mode wind3 --even "A,B,C#,D,E,F#,G#" -p=1 -t=6 | aplay -f S16_LE -c 2 -r 48000
  go run . --mode wind3 --even "C,E,G" -p=1 -t=6 | aplay -f S16_LE -c 2 -r 48000
  go run . --mode wind3 --even "C,Eb,G" -p=1 -t=6 | aplay -f S16_LE -c 2 -r 48000
  go run . -mode wind3 -octaves 1,2,3,4 -just 4/8,5/8,6/8,7/8 -a=0.1 -d=0.3 -s=0.4 -r=10 -t=2  -p=1  | aplay -f S16_LE -c 2 -r 48000
  go run . -mode wind3 -octaves 1,2,3,4 -just 4/8,5/8,6/8,7/8,9/8,11/8 -a=0.1 -d=0.3 -s=0.4 -r=10 -t=2  -p=1  | aplay -f S16_LE -c 2 -r 48000
  go run . -mode wind3 -octaves 1,2,3,7+7/12 -just 4/8,5/8,6/8,7/8,9/8,11/8 -a=0.1 -d=0.3 -s=0.4 -r=10 -t=2  -p=1  | aplay -f S16_LE -c 2 -r 48000
  go run . -mode wind3 -octaves 1,2,3 -just 4/8,5/8,6/8,7/8 -a=0.1 -d=0.3 -s=0.4 -r=10 -t=2  -p=1  | aplay -f S16_LE -c 2 -r 48000
  go run . -mode wind3 -octaves 1,2,3 -just pentatonic -a=0.1 -d=0.3 -s=0.4 -r=10 -t=2  -p=1  | aplay -f S16_LE -c 2 -r 48000

```sh
  # Direct native audio playback (Linux, macOS, Windows - no pipes needed!):
  go run . -mode wind3
  go run . -mode wind3 -midi auto
  go run . -mode midi

  # Play a Standard MIDI File with automatic GM voices & orchestral panning (default):
  go run . -midifile midifiles/vivaldi_4_stagioni_inverno_1_\(c\)pollen.mid
  go run . -midifile midifiles/widor_toccata_\(c\)shattuck.mid -midispeed 150

  # Customize stereo reverb wet mix (default 0.15):
  go run . -midifile midifiles/vivaldi_4_stagioni_inverno_1_\(c\)pollen.mid -reverb 0.3

  # Override auto-voices to use manual custom harmonics & envelope:
  go run . -midifile midifiles/widor_toccata_\(c\)shattuck.mid -midiauto=false -midih "10,5,3,2,1" -midir 0.5

  # Play MIDI file alongside stochastic windchimes:
  go run . -mode wind3 -midifile midifiles/widor_toccata_\(c\)shattuck.mid

  # Native live keyboard with custom tuning and harmonics:
  go run . -mode wind3 -midi auto -midia 0.01 -midir 0.4 -midis 0.7

  # Pipe to stdout / aplay (if explicitly desired):
  go run . -mode wind3 -out stdout | aplay -B 20000 -F 5000 -f S16_LE -c 2 -r 48000

  # Cross-compiling standalone binaries:
  make
```
