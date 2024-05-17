`go run *.go`

Program expects an `in.h264`. It can be generated like so.

```
ffmpeg -i $INPUT_FILE -an -c:v libx264 -bsf:v h264_mp4toannexb -b:v 2M -max_delay 0 -bf 0 output.h264
```

