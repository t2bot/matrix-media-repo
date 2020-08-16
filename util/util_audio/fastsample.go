package util_audio

import (
	"errors"
	"math"

	"github.com/faiface/beep"
)

func FastSampleAudio(stream beep.StreamSeekCloser, numSamples int) ([][2]float64, error) {
	everyNth := int(math.Round(float64(stream.Len()) / float64(numSamples)))
	samples := make([][2]float64, numSamples)
	totalRead := 0
	for i := range samples {
		pos := i * everyNth
		if stream.Position() != pos {
			err := stream.Seek(pos)
			if err != nil {
				return nil, errors.New("fast-sample: could not seek: " + err.Error())
			}
		}

		sample := make([][2]float64, 1)
		n, _ := stream.Stream(sample)
		if stream.Err() != nil {
			return nil, errors.New("fast-sample: could not stream: " + stream.Err().Error())
		}
		if n > 0 {
			samples[i] = sample[0]
			totalRead++
		} else {
			break
		}
	}
	if totalRead != len(samples) {
		return samples[:totalRead], nil
	}
	return samples, nil
}
