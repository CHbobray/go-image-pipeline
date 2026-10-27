package main

import (
	"fmt"
	"image"
	"os"
	"strings"

	"github.com/CHbobray/go-image-pipeline/imageprocessing"
)

type Job struct {
	InputPath string
	Image     image.Image
	OutPath   string
	Err       error // set by the first stage that fails; later stages pass the job through
}

func loadImage(paths []string) <-chan Job {
	out := make(chan Job)
	go func() {
		// For each input path create a job and add it to
		// the out channel
		for _, p := range paths {
			job := Job{InputPath: p,
				OutPath: strings.Replace(p, "images/", "images/output/", 1)}
			job.Image, job.Err = imageprocessing.Read(p)
			out <- job
		}
		close(out)
	}()
	return out
}

func resize(input <-chan Job) <-chan Job {
	out := make(chan Job)
	go func() {
		// For each input job, create a new job after resize and add it to
		// the out channel
		for job := range input { // Read from the channel
			if job.Err == nil {
				job.Image = imageprocessing.Resize(job.Image, 500, 500)
			}
			out <- job
		}
		close(out)
	}()
	return out
}

func convertToGrayscale(input <-chan Job) <-chan Job {
	out := make(chan Job)
	go func() {
		for job := range input { // Read from the channel
			if job.Err == nil {
				job.Image = imageprocessing.Grayscale(job.Image)
			}
			out <- job
		}
		close(out)
	}()
	return out
}

func saveImage(input <-chan Job) <-chan Job {
	out := make(chan Job)
	go func() {
		for job := range input { // Read from the channel
			if job.Err == nil {
				job.Err = imageprocessing.Write(job.OutPath, job.Image)
			}
			out <- job
		}
		close(out)
	}()
	return out
}

func main() {

	imagePaths := []string{"images/retina.jpg",
		"images/hubble_deep_field.jpg",
		"images/spacex_rocket.jpg",
		"images/coffee.png",
	}
	if err := os.MkdirAll("images/output", 0o755); err != nil {
		fmt.Println("Failed:", err)
		os.Exit(1)
	}

	channel1 := loadImage(imagePaths)
	channel2 := resize(channel1)
	channel3 := convertToGrayscale(channel2)
	writeResults := saveImage(channel3)

	failures := 0
	for job := range writeResults {
		if job.Err != nil {
			fmt.Println("Failed:", job.Err)
			failures++
		} else {
			fmt.Println("Success!", job.OutPath)
		}
	}
	if failures > 0 {
		os.Exit(1)
	}
}
