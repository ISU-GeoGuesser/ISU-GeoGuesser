package main

import (
	"image"
	_ "image/jpeg"
	_ "image/png"
	"isu-geoguesser/config"
	"log"
	"net/http"
	"os"

	gometadata "github.com/FlavioCFOliveira/GoMetadata"
	"github.com/gin-gonic/gin"
	"github.com/ryanbekhen/go-webp"

	db "isu-geoguesser/database"
)

func uploadLocation(c *gin.Context) {
	name := c.PostForm("name")
	if name == "" {
		log.Println("error: Location name is required")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Location name is required"})
		return
	}

	fileHeader, err := c.FormFile("image")
	if err != nil {
		log.Println("error: No image received: ", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "No image received: " + err.Error()})
		return
	}

	tmpDst := "./tmp/" + fileHeader.Filename
	if err := c.SaveUploadedFile(fileHeader, tmpDst); err != nil {
		log.Println("error: Failed to save uploaded file: ", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save uploaded file" + err.Error()})
		return
	}

	m, err := gometadata.ReadFile(tmpDst)
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

	file, err := os.Open(tmpDst)
	if err != nil {
		log.Println("error: Failed to open temporary file: ", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open temporary file"})
		return
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		log.Println("error: Failed to decode image: ", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode image: " + err.Error()})
		return
	}

	output := config.IMAGE_DIR + fileHeader.Filename + ".webp"
	outFile, err := os.Create(output)
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
		return
	}

	_, err = db.DB.Exec(
		db.INSERT_LOCATION,
		output, name, lat, lon,
	)
	if err != nil {
		log.Println("error: Failed to save location: ", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save location"})
		return
	}

	fileInfo, err := os.Stat(output)
	log.Println("Location uploaded successfully: ", name, "\nfile: ", output, "\ncoordinates: ", lat, lon, "\nsize: ", fileInfo.Size()/1024, "KB")

	c.JSON(http.StatusOK, gin.H{
		"message":   "Image uploaded successfully",
		"name":      name,
		"filename":  output,
		"latitude":  lat,
		"longitude": lon,
	})
}
