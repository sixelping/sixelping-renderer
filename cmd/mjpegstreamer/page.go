package main

import (
	"html/template"
	"log"
	"net/http"
)

func pageHandler(w http.ResponseWriter, r *http.Request) {
	pageTemplate, err := template.New("index").Parse(`
<html>
<head>
<style>
html, body, img {
	min-width: {{.Width}};
	min-height: {{.Height}};
	margin: 0;
	padding: 0;
}

img {
	position: absolute;
	left: 0;
	top: 0;
}

</style>
</head>
<body>
<img src="/stream.mjpeg" />
</body>
</html>
`)
	if err == nil {
		var dets struct {
			Width  uint32
			Height uint32
		}
		dets.Width = canvasParameters.GetWidth()
		dets.Height = canvasParameters.GetHeight()
		pageTemplate.Execute(w, dets)
	} else {
		log.Fatalf("Template invalid: %v", err)
	}
}
