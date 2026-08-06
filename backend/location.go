package main

import (
	"image"
	_ "image/jpeg"
	_ "image/png"
	"isu-geoguesser/config"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	gometadata "github.com/FlavioCFOliveira/GoMetadata"
	"github.com/gin-gonic/gin"
	"github.com/ryanbekhen/go-webp"

	db "isu-geoguesser/database"
)

const locationFileMode = 0755

func uploadLocation(c *gin.Context) {
	name := c.PostForm("name")
	if name == "" {
		log.Println("error: Location name is required")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Location name is required"})
		return
	}

	fileHeader, err := c.FormFile("image")
	fileName := filepath.Base(fileHeader.Filename)
	if err != nil {
		log.Println("error: No image received: ", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "No image received: " + err.Error()})
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		log.Println("error: Failed to open temporary file: ", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open temporary file"})
		return
	}
	defer file.Close()

	m, err := gometadata.Read(file)
	if err != nil {
		log.Println("error: Failed to read metadata: ", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read metadata: " + err.Error()})
		return
	}

	lat, lon := 0.0, 0.0
	if newLat, newLon, ok := m.GPS(); !ok {
		log.Println("error: Image has no location metadata")
		c.JSON(http.StatusBadRequest, gin.H{"error": "image has no GPS metadata"})
		return
	} else {
		lat = newLat
		lon = newLon
	}

	file.Seek(0, 0)
	img, _, err := image.Decode(file)
	if err != nil {
		log.Println("error: Failed to decode image: ", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode image: " + err.Error()})
		return
	}

	os.MkdirAll(config.IMAGE_DIR, locationFileMode)
	outputFilename := strings.TrimSuffix(fileName, filepath.Ext(fileName)) + ".webp"
	outputPath := filepath.Join(config.IMAGE_DIR, outputFilename)
	outFile, err := os.OpenFile(outputPath, os.O_CREATE|os.O_EXCL|os.O_RDWR, locationFileMode)
	if err != nil {
		log.Println("error: Failed to create output file: ", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create output file: " + err.Error()})
		return
	}
	defer outFile.Close()

	err = webp.Encode(img, 75, outFile)
	if err != nil {
		log.Println("error: Failed to encode image to WebP: ", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encode image to WebP: " + err.Error()})
		os.Remove(outputPath)
		return
	}

	_, err = db.DB.Exec(
		db.INSERT_LOCATION,
		outputFilename, name, lat, lon,
	)
	if err != nil {
		log.Println("error: Failed to save location: ", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save location"})
		os.Remove(outputPath)
		return
	}

	outInfo, _ := outFile.Stat()
	log.Println(
		"Location uploaded successfully:", name,
		"\n\tfile:", outputFilename,
		"\n\tcoords:", lat, lon,
		"\n\tsize:", outInfo.Size()/1024, "KB")

	c.JSON(http.StatusOK, gin.H{
		"message":   "Image uploaded successfully",
		"name":      name,
		"filename":  outputFilename,
		"latitude":  lat,
		"longitude": lon,
	})
}
