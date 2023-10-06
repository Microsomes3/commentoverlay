package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

type Job struct {
	CommentVideoUrl string
	VideoUrl        string
	UploadId        string
}

func getJob() *Job {

	j := &Job{}

	//ask for comment video url

	j.CommentVideoUrl = os.Getenv("COMMENT_VIDEO_URL")
	j.VideoUrl = os.Getenv("VIDEO_URL")
	j.UploadId = "upload1"

	return j

}

func DownloadFile(fileName string, link string, w *sync.WaitGroup) bool {
	defer w.Done()
	resp, err := http.Get(link)

	if err != nil {
		return false
	}

	defer resp.Body.Close()

	out, err := os.Create(fileName)

	if err != nil {
		return false
	}

	defer out.Close()

	_, err = io.Copy(out, resp.Body)

	if err != nil {
		return false
	}

	return true

}

func (j *Job) DownloadVideos() {
	//download videos to perform overlay processing

	//download comment video

	syn := sync.WaitGroup{}
	syn.Add(2)
	go DownloadFile("comment.mp4", j.CommentVideoUrl, &syn)
	go DownloadFile("video.mp4", j.VideoUrl, &syn)

	syn.Wait()

	fmt.Println("Downloaded videos")
}

func (j *Job) GetVideoDuration() (int64, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", "video.mp4")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0, err
	}

	// The ffprobe output contains the duration in seconds as a string.
	durationStr := strings.TrimSpace(string(output))

	// Convert the duration string to an int64.
	duration, err := strconv.ParseFloat(durationStr, 64)
	if err != nil {
		return 0, err
	}

	// Convert the duration to seconds and return it as an int64.
	durationInSeconds := int64(duration)
	return durationInSeconds, nil
}

func (j *Job) GetCommentDuration() (int64, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", "comment.mp4")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0, err
	}

	// The ffprobe output contains the duration in seconds as a string.
	durationStr := strings.TrimSpace(string(output))

	// Convert the duration string to an int64.
	duration, err := strconv.ParseFloat(durationStr, 64)
	if err != nil {
		return 0, err
	}

	// Convert the duration to seconds and return it as an int64.
	durationInSeconds := int64(duration)
	return durationInSeconds, nil
}

func (j *Job) CropCommentVideo() bool {
	cmd := exec.Command("ffmpeg", "-i", "comment.mp4", "-filter:v", "crop=600:400:0:0", "-c:a", "copy", "comment_cropped.mp4")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Start()
	if err != nil {
		fmt.Println("Error starting command:", err)
		return false
	}

	err = cmd.Wait()
	if err != nil {
		fmt.Println("Error waiting for command:", err)
		return false
	}

	return true
}

func (j *Job) PerformProcessing() {
	cmd := exec.Command("ffmpeg", "-i", "video.mp4", "-i", "comment_cropped.mp4", "-filter_complex", "[1]format=rgba,colorchannelmixer=aa=0.5[ovrl];[0][ovrl]overlay=x=(W-w)+20:y=100", "-preset", "ultrafast", "output.mp4")

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd.Start()

	err := cmd.Wait()
	if err != nil {
		fmt.Println("Error waiting for command:", err)
		return
	}

	fmt.Println("Processing done")
}

func printOutput(stdout io.Reader) {
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		fmt.Println(scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading output:", err)
	}
}

func (*Job) UploadVideo(uploadId string) {

	o, err := os.Open("output.mp4")

	if err != nil {
		fmt.Println(err)
		return
	}

	defer o.Close()

	uploader := &DLPUploader{}

	p, err := uploader.UploadFile(o, uploadId+"ex_output.mp4", "1")

	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(p)

}

func (job *Job) analyseCommentVideo() bool {

	dur, _ := job.GetVideoDuration()
	comdur, _ := job.GetCommentDuration()

	if dur == comdur {
		return true
	}

	//we need to crop

	var toCropLength int64 = 0

	if dur > comdur {
		toCropLength = comdur
	} else {
		toCropLength = dur
	}

	fmt.Println("Crop length", toCropLength)

	job.CropVideo("video.mp4", toCropLength)

	job.CropVideo("comment.mp4", toCropLength)

	return true

}

func (job *Job) shrinkOutput() bool {
	inputFile := "output.mp4"
	outputFile := "output_compressed.mp4"

	cmd := exec.Command("ffmpeg", "-i", inputFile, "-vf", "scale=1280:720", "-c:v", "libx264", "-crf", "23", "-c:a", "aac", "-b:a", "128k", outputFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Println("Error:", err)
		return false
	}

	//rename output file

	err := os.Rename(outputFile, inputFile)

	if err != nil {
		fmt.Println(err)
		return false
	}

	fmt.Println("Video compression completed successfully!")
	return true
}

func (job *Job) CropVideo(videoFileName string, cropDuration int64) error {
	// Define the output file name for the cropped video.
	outputFileName := "cropped_" + videoFileName

	// Build the ffmpeg command to crop the video.
	cmd := exec.Command("ffmpeg",
		"-i", videoFileName, // Input video file.
		"-ss", "0", // Start from the beginning.
		"-t", fmt.Sprintf("%d", cropDuration), // Crop to the specified duration.
		"-c:v", "copy", // Copy video codec.
		"-c:a", "copy", // Copy audio codec.
		"-y",           // Overwrite output file if it exists.
		outputFileName, // Output file name.
	)

	// Execute the ffmpeg command.
	if err := cmd.Run(); err != nil {
		return err
	}

	//rename the file using mv

	err := os.Rename(outputFileName, videoFileName)

	if err != nil {
		return err
	}

	return nil
}

func printHelp() {
	fmt.Println("Usage: ")
	fmt.Println("go run main.go -video <video url> -comment <comment video url> -uploadId <upload id>")
}

func main() {

	// fmt.Println("Starting processing")

	// videoUrl := flag.String("video", "", "Video url")
	// commentUrl := flag.String("comment", "", "Comment video url")
	// uploadId := flag.String("uploadId", "", "Upload id")

	// flag.Parse()

	// if *videoUrl == "" {
	// 	fmt.Println("Video url is required")
	// 	printHelp()
	// 	return
	// }

	// if *commentUrl == "" {
	// 	fmt.Println("Comment video url is required")
	// 	printHelp()
	// 	return
	// }

	// if *uploadId == "" {
	// 	fmt.Println("Upload id is required")
	// 	printHelp()

	// 	return
	// }

	// job := &Job{
	// 	CommentVideoUrl: *commentUrl,
	// 	VideoUrl:        *videoUrl,
	// }

	job := getJob()

	fmt.Println("downloading video")
	job.DownloadVideos()
	fmt.Println("downloaded video")

	job.analyseCommentVideo()

	fmt.Println("cropping comment video")
	job.CropCommentVideo()
	fmt.Println("cropped comment video")

	fmt.Println("performing processing")
	job.PerformProcessing()
	fmt.Println("processing done")

	// job.shrinkOutput()

	fmt.Println("uploading video")
	job.UploadVideo(job.UploadId)
	fmt.Println("uploaded video")

}
