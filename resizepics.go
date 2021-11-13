package main

import (
	"bufio"
	"flag"
	"fmt"
	"image"
	"image/color"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/disintegration/imaging"
	"github.com/joseg2/imageformat"
	"github.com/pieterclaerhout/go-log"
)

func main() {
	// cli flags
	var crop_wide_pics bool
	var source_path string
	var dest_path string
	var base_width float64
	var base_height float64

	flag.BoolVar(&crop_wide_pics, "crop_wide_pics", false, "(Optional) Crop wide pictures to base ratio, omitting this flag maintains original ratio by padding images")
	flag.Float64Var(&base_width, "dst-width", 800.0, "(Optional) Width of the transformed images")
	flag.Float64Var(&base_height, "dst-height", 600.0, "(Optional) Height of the transformed images")

	flag.StringVar(&source_path, "source", "/undefined_source", "(Required) Full path of the source images to transform")
	flag.StringVar(&dest_path, "destination", "/undefined_destination", "(Required) Full path where the transformed images will be created")
	flag.Parse()
	fmt.Printf("Flag values:\n   crop_wide_pics: %v\n   source: %v\n   destination: %v\n   dst-width: %v\n   dst-height: %v\n\n", crop_wide_pics, source_path, dest_path, base_width, base_height)

	// defaults
	var base_ratio float64 = base_width / base_height
	var old_ratio float64 = 0.00
	var position string = "horizontal"

	if source_path == "/undefined_source" {
		log.Fatal("Missing flag and value: -source /full/path/to/source/image/files")
	}
	if dest_path == "/undefined_destination" {
		log.Fatal("Missing flag and value: -destination /full/path/to/destination/image/files")
	}

	// Read list of files from filesystem
	var list_of_files []string
	var errFiles error

	errFiles = filepath.Walk(source_path, imageformat.Visit(&list_of_files))
	if errFiles != nil {
		panic(errFiles)
	}

	for _, file := range list_of_files[1:] {
		fmt.Println("file is: ", file)
	}
	fmt.Println("Range has been established")

	// Print the log timestamps
	log.PrintTimestamp = true

	// The command you want to run along with the argument
	app := "file"
	cmd := exec.Command(app, list_of_files[1:]...)

	/* sample output
	/path/xxx/Pictures/star-wars-backgrounds-31.jpg:   JPEG image data, Exif standard: [TIFF image data, little-endian, direntries=0], baseline, precision 8, 1920x1080, components 3
	/path/xxx/Pictures/file-01.jpg: JPEG image data, JFIF standard 1.01, resolution (DPI), density 300x300, segment length 16, Exif Standard: [TIFF image data, little-endian, direntries=7], baseline, precision 8, 5828x3891, components 3
	*/

	// Get a pipe to read from standard out
	r, _ := cmd.StdoutPipe()

	// Use the same pipe for standard error
	cmd.Stderr = cmd.Stdout

	// Make a new channel which will be used to ensure we get all output
	done := make(chan struct{})

	// Create a scanner which scans r in a line-by-line fashion
	scanner := bufio.NewScanner(r)

	// Use the scanner to scan the output line by line and log it
	// It's running in a goroutine so that it doesn't block
	go func() {

		// Read line by line and process it
		for scanner.Scan() {
			var orientation string = "unknown"
			var rotated bool = false

			line := scanner.Text()
			log.Debug(line)

			regex_dimensions := `(\b\d\d\d+x\d\d\d+\b)`
			re_size := regexp.MustCompile(regex_dimensions)
			found_size := re_size.FindAllString(line, -1)
			size := found_size[len(found_size)-1]

			re_dim := regexp.MustCompile(`\d+`)
			matches_dim := re_dim.FindAllString(size, -1)

			str_width := matches_dim[0]
			int_width, err := strconv.Atoi(str_width)
			if err != nil {
				fmt.Println(err)
				os.Exit(2)
			}
			flt_width := float64(int_width)

			str_height := matches_dim[1]
			int_height, err := strconv.Atoi(str_height)
			if err != nil {
				fmt.Println(err)
				os.Exit(2)
			}
			flt_height := float64(int_height)

			// Filepath extraction
			regex_filepath := `(\/.*\.[jJ][pP][eE]*[gG])`
			re_filepath := regexp.MustCompile(regex_filepath)
			found_filepath := re_filepath.FindAllString(line, -1)
			filepath := found_filepath[0]
			fmt.Printf("\n\nProcessing file %s: \n", filepath)

			// Filename extraction
			regex_filename := `([^\/]*\.[jJ][pP][eE]*[gG])`
			re_filename := regexp.MustCompile(regex_filename)
			found_filename := re_filename.FindAllString(line, -1)
			filename := found_filename[0]

			// Determine orientation (an exif tag) and position (vertical or horizontal) from image metadata
			regex_orientation := `(orientation=)([\w-]+)`
			re_orientation := regexp.MustCompile(regex_orientation)
			found_orientation := re_orientation.FindAllStringSubmatch(line, -1)
			if len(found_orientation) > 0 {
				log.Debug("\n   Found orientation tag!!!\n\n   size is: %d\n\n   value is: %q\n\n", len(found_orientation), found_orientation[0][2])
				orientation = found_orientation[0][2]
			} else {
				log.Debug("\n   DID NOT find orientation tag!!!\n\n")
			}

			if orientation == "lower-left" || orientation == "upper-right" {
				position = "vertical"
				rotated = true
			} else {
				// Determine image orientation and position from image Width x Height
				if orientation == "unknown" {

					//position = "unknown"
					rotated = false

					if int_height <= int_width {
						position = "horizontal"
					} else {
						position = "vertical"
					}

				} else {
					// Images with any other orientations are considered to be horizontal
					position = "horizontal"
					rotated = false
				}
			}

			// Resize canvas of images
			var new_int_width int
			var new_str_width string
			var new_int_height int
			var new_str_height string
			old_ratio = flt_width / flt_height

			if position == "vertical" { // portrait-mode images that are currently sideways, to be rotated later
				new_int_width = imageformat.RoundToInt(flt_width * base_ratio) // calculated with the future height (pre rotated witdth)
				new_str_width = strconv.Itoa(new_int_width)
				new_int_height = int_width
				new_str_height = strconv.Itoa(new_int_height)
				fmt.Printf("  - position is vertical\nposition: %s\nnew_str_width: %s\nnew_str_height: %s\nold_ratio: %f\n", position, new_str_width, new_str_height, old_ratio)
			} else {
				if crop_wide_pics == true {
					new_int_width = imageformat.RoundToInt(flt_height * base_ratio) // width for crop to base_ratio, ie. chop the sides of wider images
					new_int_height = int_height                                     // height for crop to base_ratio, ie. chop the sides of wider images
				} else {
					new_int_width = int_width                                       // keep wide images in their original framing, no cropping is done
					new_int_height = imageformat.RoundToInt(flt_width / base_ratio) // keep wide images in their framing, lengthen canvas to fit base_ratio
				}
				new_str_width = strconv.Itoa(new_int_width)
				if err != nil {
					fmt.Println(err)
					os.Exit(2)
				}
				new_str_height = strconv.Itoa(new_int_height)
				if err != nil {
					fmt.Println(err)
					os.Exit(2)
				}
				fmt.Printf("  - position is NOT vertical\nposition: %s\nnew_str_width: %s\nnew_str_height: %s\nold_ratio: %f\n", position, new_str_width, new_str_height, old_ratio)
			}

			new_size := new_str_width + "x" + new_str_height

			var thispic imageformat.Photo
			thispic.Filepath = filepath
			thispic.Orientation = orientation
			thispic.Ratio = old_ratio
			thispic.Width = int_width

			processed := imageformat.Filter(thispic, base_ratio, base_width, crop_wide_pics)

			fmt.Println("      old_ratio is ", old_ratio)

			log.Info("Size=", size, " Width=", str_width, " Height=", str_height, " Orientation=", orientation, " Position=", position, " Rotated=", rotated, "Ratio=", old_ratio, "New size", new_size)
			log.Info("    OPERATIONS 2B DONE - 1.Rotate:", processed.Rotate, "2.Centercrop:", processed.Centercrop, "3.Fillsides:", processed.Fillsides, "4.Scaledown:", processed.Scaledown)

			fmt.Println("Applying transformations on file: ", filename)
			if processed.Rotate == "none" && processed.Centercrop == false && processed.Fillsides == false && processed.Scaledown == false {
				fmt.Println("No changes required to image")
			} else {

				src, err := imaging.Open(filepath)
				if err != nil {
					log.Fatal("Failed to open image: %v", err)
				}

				if processed.Rotate == "acw" {
					fmt.Println(" - rotate ccw (90)")
					src = imaging.Rotate90(src)
				}
				if processed.Rotate == "cw" {
					fmt.Println(" - rotate cw (270)")
					src = imaging.Rotate270(src)
				}
				if processed.Centercrop == true {
					fmt.Println(" - cropcenter ", new_int_width, "x", new_int_height, ": ", filename)
					src = imaging.CropCenter(src, new_int_width, new_int_height)
				}
				if processed.Fillsides == true {
					fmt.Println(" - fillsides")
					canvas := imaging.New(new_int_width, new_int_height, color.NRGBA{0, 0, 0, 0})
					src = imaging.PasteCenter(canvas, src)
				}
				if processed.Scaledown == true {
					fmt.Println(" - scaledown")
					src = imaging.Resize(src, int(base_width), 0, imaging.Lanczos)
				}
				dst := imaging.New(int(base_width), int(base_height), color.NRGBA{0, 0, 0, 0})
				dst = imaging.Paste(dst, src, image.Pt(0, 0))
				err = imaging.Save(dst, dest_path+"/"+filename)
				if err != nil {
					log.Fatalf("Failed to save image: %v", err)
				}

			}
		}

		// We're all done, unblock the channel
		done <- struct{}{}

	}()

	// Start the command and check for errors
	err := cmd.Start()
	log.CheckError(err)

	// Wait for all output to be processed
	<-done

	// Wait for the command to finish
	err = cmd.Wait()
	log.CheckError(err)
	fmt.Println("have a nice day")

}
