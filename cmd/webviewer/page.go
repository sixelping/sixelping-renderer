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
	background-image: url('/image.png');
}

img {
	position: absolute;
	left: 0;
	top: 0;
}

</style>
<script>
function refresher(img) {
	var randomId = new Date().getTime();
	img.src = "/image.png?random=" + randomId;
}

function i1() {
	var img1 = document.getElementById("img1");
	var img2 = document.getElementById("img2");
	img1.style.display = "block";
	img2.style.display = "hidden";
	img2.onload = i2;
	refresher(img2);
}

function i2() {
	var img1 = document.getElementById("img1");
	var img2 = document.getElementById("img2");
	img2.style.display = "block";
	img1.style.display = "hidden";
	img1.onload = i1;
	refresher(img1);
}
</script>
</head>
<body>
<img id="img1" src="/image.png" onload="i1()" />
<img id="img2" />
</body>
</html>
`)
	if err == nil {
		var dets struct {
			Width  int
			Height int
			Fps    int
		}
		dets.Width = canvas.Bounds().Size().X
		dets.Height = canvas.Bounds().Size().Y
		dets.Fps = fps
		pageTemplate.Execute(w, dets)
	} else {
		log.Fatalf("Template invalid: %v", err)
	}
}
