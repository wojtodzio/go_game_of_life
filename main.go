package main

import (
	"fmt"
	"log"
	"runtime"
	"strings"
	"math/rand"
	"time"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/glfw/v3.2/glfw"
)

const (
	width  = 500
	height = 500

	rows = 50
	columns = 50

	scaleX = 2.0 / rows
	scaleY = 2.0 / columns

	threshold = 0.15

	fps = 20

	vertexShaderSource = `
		#version 410
		in vec3 vp;
		void main() {
			gl_Position = vec4(vp, 1.0);
		}
	` + "\x00"

	fragmentShaderSource = `
		#version 410
		out vec4 frag_colour;
		void main() {
			frag_colour = vec4(1, 1, 1, 1);
		}
	` + "\x00"
)

type point [3]float32

type triangle [3]point

type square [2]triangle

func newSquare(a, b, c, d point) *square {
	t1 := triangle{a, b, c}
	t2 := triangle{b, c, d}
	return &square{t1, t2}
}

type Shape interface {
	flatPoints() []float32
}

func (t *triangle) flatPoints() []float32 {
	var flatPoints []float32
	for _, point := range t {
		flatPoints = append(flatPoints, point[:]...)
	}
	return flatPoints
}

func (s *square) flatPoints() []float32 {
	return append(s[0].flatPoints(), s[1].flatPoints()...)
}

type cell struct {
	drawable uint32

	alive     bool
	aliveNext bool

	x int
	y int
}

func (c *cell) checkState(cells [][]*cell) {
    c.alive = c.aliveNext
    c.aliveNext = c.alive

    liveCount := c.liveNeighbors(cells)
    if c.alive {
        // 1. Any live cell with fewer than two live neighbours dies, as if caused by underpopulation.
        if liveCount < 2 {
            c.aliveNext = false
        }

        // 2. Any live cell with two or three live neighbours lives on to the next generation.
        if liveCount == 2 || liveCount == 3 {
            c.aliveNext = true
        }

        // 3. Any live cell with more than three live neighbours dies, as if by overpopulation.
        if liveCount > 3 {
            c.aliveNext = false
        }
    } else {
        // 4. Any dead cell with exactly three live neighbours becomes a live cell, as if by reproduction.
        if liveCount == 3 {
            c.aliveNext = true
        }
    }
}

func (c *cell) liveNeighbors(cells [][]*cell) int {
    var liveCount int
    add := func(x, y int) {
        if x == len(cells) {
            x = 0
        } else if x == -1 {
            x = len(cells) - 1
        }
        if y == len(cells[x]) {
            y = 0
        } else if y == -1 {
            y = len(cells[x]) - 1
        }

        if cells[x][y].alive {
            liveCount++
        }
    }

    add(c.x - 1, c.y)
    add(c.x + 1, c.y)
    add(c.x, c.y + 1)
    add(c.x, c.y - 1)
    add(c.x - 1, c.y + 1)
    add(c.x + 1, c.y + 1)
    add(c.x - 1, c.y - 1)
    add(c.x + 1, c.y - 1)

    return liveCount
}

func (c *cell) draw() {
	if !c.alive {
		return
	}

	gl.BindVertexArray(c.drawable)
	gl.DrawArrays(gl.TRIANGLES, 0, 6)
}

func newCell(xInt, yInt int) *cell {
	x := float32(xInt)
	y := float32(yInt)
	a := point{scaleX * x - 1, scaleY * y - 1, 0}
	b := point{scaleX * x - 1 + scaleX, scaleY * y - 1, 0}
	c := point{scaleX * x - 1, scaleY * y - 1 + scaleY, 0}
	d := point{scaleX * x - 1 + scaleX, scaleY * y - 1 + scaleY, 0}

	return &cell{
		drawable: makeVao(newSquare(a, b, c, d)),

		x: xInt,
		y: yInt,
	}
}

func makeBoard() [][]*cell  {
	rand.Seed(time.Now().UnixNano())

	cells := make([][]*cell, rows, rows)
	for x := 0; x < rows; x++ {
		for y := 0; y < columns; y++ {
			c := newCell(x, y)

			c.alive = rand.Float64() < threshold
			c.aliveNext = c.alive

			cells[x] = append(cells[x], c)
		}
	}

	return cells
}

func main() {
	runtime.LockOSThread()

	window := initGlfw()
	defer glfw.Terminate()

	program := initOpenGL()

	board := makeBoard()

	for !window.ShouldClose() {
		t := time.Now()

		for x := range board {
			for _, c := range board[x] {
				c.checkState(board)
			}
		}

		draw(board, window, program)

		time.Sleep(time.Second/time.Duration(fps) - time.Since(t))
	}
}

// initGlfw initializes glfw and returns a Window to use.
func initGlfw() *glfw.Window {
	if err := glfw.Init(); err != nil {
		panic(err)
	}

	glfw.WindowHint(glfw.Resizable, glfw.False)
	glfw.WindowHint(glfw.ContextVersionMajor, 4)
	glfw.WindowHint(glfw.ContextVersionMinor, 1)
	glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)
	glfw.WindowHint(glfw.OpenGLForwardCompatible, glfw.True)

	window, err := glfw.CreateWindow(width, height, "Conway's Game of Life", nil, nil)
	if err != nil {
		panic(err)
	}
	window.MakeContextCurrent()

	return window
}

func initOpenGL() uint32 {
	if err := gl.Init(); err != nil {
		panic(err)
	}
	version := gl.GoStr(gl.GetString(gl.VERSION))
	log.Println("OpenGL version", version)

	vertexShader, err := compileShader(vertexShaderSource, gl.VERTEX_SHADER)
	if err != nil {
		panic(err)
	}
	fragmentShader, err := compileShader(fragmentShaderSource, gl.FRAGMENT_SHADER)
	if err != nil {
		panic(err)
	}

	prog := gl.CreateProgram()
	gl.AttachShader(prog, vertexShader)
	gl.AttachShader(prog, fragmentShader)
	gl.LinkProgram(prog)
	return prog
}

func draw(cells [][]*cell, window *glfw.Window, program uint32) {
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)
	gl.UseProgram(program)

	for x := range cells {
		for _, c := range cells[x] {
			c.draw()
		}
	}

	glfw.PollEvents()
	window.SwapBuffers()
}

func makeVao(shape Shape) uint32 {
	points := shape.flatPoints()
	var vbo uint32
	gl.GenBuffers(1, &vbo)
	gl.BindBuffer(gl.ARRAY_BUFFER, vbo)
	gl.BufferData(gl.ARRAY_BUFFER, 4 * len(points), gl.Ptr(points), gl.STATIC_DRAW)

	var vao uint32
	gl.GenVertexArrays(1, &vao)
	gl.BindVertexArray(vao)
	gl.EnableVertexAttribArray(0)
	gl.BindBuffer(gl.ARRAY_BUFFER, vbo)
	gl.VertexAttribPointer(0, 3, gl.FLOAT, false, 0, nil)

	return vao
}

func compileShader(source string, shaderType uint32) (uint32, error) {
	shader := gl.CreateShader(shaderType)

	csources, free := gl.Strs(source)
	gl.ShaderSource(shader, 1, csources, nil)
	free()
	gl.CompileShader(shader)

	var status int32
	gl.GetShaderiv(shader, gl.COMPILE_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetShaderiv(shader, gl.INFO_LOG_LENGTH, &logLength)

		log := strings.Repeat("\x00", int(logLength+1))
		gl.GetShaderInfoLog(shader, logLength, nil, gl.Str(log))

		return 0, fmt.Errorf("failed to compile %v: %v", source, log)
	}

	return shader, nil
}
