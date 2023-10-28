package main

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const polarShader = `package main

func Fragment(position vec4, texCoord vec2, color vec4) vec4 {
	pi := 3.14
	dist := 1.0
	resolution := vec2(180.0, 180.0)

	if position.y > 1 {
		discard()
	}

	for y := 0.0; y < 180.0; y += 1.0 {
		// Rectangular to polar filter.
		norm := vec2(texCoord.s, y / resolution.y) * 2.0 - 1.0
		theta := pi * 1.5 + norm.x * pi
		r := (1.0 + norm.y) * 0.5

		// Coordinate to sample from the occlusion map.
		coord := vec2(-r * sin(theta), -r * cos(theta)) / 2.0 + 0.5

		// Sample occlusion map.
		data := imageSrc0UnsafeAt(coord)

		// Distance from the top.
		dst := y / resolution.y

		// If alpha > 0.75, the ray hit.
		if data.a > 0.75 {
			dist = min(dist, dst)
			break
		}
	}

	return vec4(vec3(dist), 1.0)
}
`

const shadowShader = `package main

func Fragment(position vec4, texCoord vec2, color vec4) vec4 {
	pi := 3.14
	resolution := vec2(180.0, 180.0)

	col := vec4(0.0, 0.0, 0.0, 1.0)
	// Rectangular to polar.
	//norm := texCoord.st * 2.0 - 1.0
	norm := vec2(texCoord.s * 2.0 - 1.0, texCoord.t * -2.0 + 1.0) // This doesn't seem right, but Y is inverted otherwise...
	theta := atan2(norm.y, norm.x)
	r := length(norm)
	coord := (theta + pi) / (2.0 * pi)

	// Sample shadow texture
	tc := vec2(coord, 0)

	// Get center???
	center := step(r, imageSrc0UnsafeAt(vec2(tc.x, tc.y)).r)

	// Fade stuff.
	blur := (1. / resolution.x) * smoothstep(0., 1., r)

	// Gaussian blur
	sum := 0.0

	sum += step(r, imageSrc0UnsafeAt(vec2(tc.x - 4.0*blur, tc.y)).r) * 0.05
	sum += step(r, imageSrc0UnsafeAt(vec2(tc.x - 3.0*blur, tc.y)).r) * 0.09
	sum += step(r, imageSrc0UnsafeAt(vec2(tc.x - 2.0*blur, tc.y)).r) * 0.12
	sum += step(r, imageSrc0UnsafeAt(vec2(tc.x - 1.0*blur, tc.y)).r) * 0.15

	sum += center * 0.16

	sum += step(r, imageSrc0UnsafeAt(vec2(tc.x + 1.0*blur, tc.y)).r) * 0.15
	sum += step(r, imageSrc0UnsafeAt(vec2(tc.x + 2.0*blur, tc.y)).r) * 0.12
	sum += step(r, imageSrc0UnsafeAt(vec2(tc.x + 3.0*blur, tc.y)).r) * 0.09
	sum += step(r, imageSrc0UnsafeAt(vec2(tc.x + 4.0*blur, tc.y)).r) * 0.05

	lit := mix(center, sum, 1.0)

	return col * vec4(vec3(1.0), (1.0-lit) * smoothstep(1.0, 0.0, r))
}
`

var openBox = [][2]float32{
	{150, 150},
	{150, 300},
	{300, 300},
	{300, 150},
}

var circle = [2]float32{
	200, 200,
}

type Game struct {
	init bool

	WorldImage *ebiten.Image

	OcclusionImage *ebiten.Image

	PolarShader   *ebiten.Shader
	PolarMapImage *ebiten.Image

	FinalImage *ebiten.Image

	ShadowShader        *ebiten.Shader
	ShadowShaderOptions *ebiten.DrawRectShaderOptions
}

func (g *Game) Init() (err error) {
	g.WorldImage = ebiten.NewImage(400, 400)
	g.OcclusionImage = ebiten.NewImage(400, 400)
	g.PolarMapImage = ebiten.NewImage(400, 400)
	g.FinalImage = ebiten.NewImage(400, 400)
	g.ShadowShaderOptions = &ebiten.DrawRectShaderOptions{
		Uniforms: make(map[string]interface{}),
	}
	if g.PolarShader, err = ebiten.NewShader([]byte(polarShader)); err != nil {
		return err
	}
	if g.ShadowShader, err = ebiten.NewShader([]byte(shadowShader)); err != nil {
		return err
	}

	ebiten.SetWindowSize(801, 801)

	return nil
}

func (g *Game) Update() error {
	if !g.init {
		g.init = true
		if err := g.Init(); err != nil {
			return err
		}
	}

	if ebiten.IsKeyPressed(ebiten.KeyLeft) || ebiten.IsKeyPressed(ebiten.KeyA) {
		circle[0]--
	}
	if ebiten.IsKeyPressed(ebiten.KeyRight) || ebiten.IsKeyPressed(ebiten.KeyD) {
		circle[0]++
	}
	if ebiten.IsKeyPressed(ebiten.KeyUp) || ebiten.IsKeyPressed(ebiten.KeyW) {
		circle[1]--
	}
	if ebiten.IsKeyPressed(ebiten.KeyDown) || ebiten.IsKeyPressed(ebiten.KeyS) {
		circle[1]++
	}

	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	// Draw our world.
	{
		// Clear the background.
		g.WorldImage.Fill(color.NRGBA{255, 255, 255, 255})

		// Draw our walls.
		for i, p := range openBox {
			var b [2]float32
			if i+1 >= len(openBox) {
				b = p
			} else {
				b = openBox[i+1]
			}
			vector.StrokeLine(g.WorldImage, p[0], p[1], b[0], b[1], 6, color.NRGBA{255, 0, 0, 255}, true)
		}

		// Draw our circle.
		vector.DrawFilledCircle(g.WorldImage, circle[0], circle[1], 30, color.NRGBA{0, 255, 0, 255}, true)
	}

	// Draw our "shadows" to our occlusion texture.
	{
		g.OcclusionImage.Fill(color.NRGBA{255, 255, 255, 255})
		// Draw our walls.
		for i, p := range openBox {
			var b [2]float32
			if i+1 >= len(openBox) {
				b = p
			} else {
				b = openBox[i+1]
			}
			vector.StrokeLine(g.OcclusionImage, p[0]+1, p[1]+1, b[0]+1, b[1]+1, 4, color.NRGBA{0, 0, 0, 255}, true)
		}
	}

	// Generate our "1D" shadow map. Note that this draws a _single_ pixel line to represent the shadow coordinates.
	{
		g.PolarMapImage.Clear()
		g.PolarMapImage.DrawRectShader(g.OcclusionImage.Bounds().Dx(), g.OcclusionImage.Bounds().Dy(), g.PolarShader, &ebiten.DrawRectShaderOptions{
			Images: [4]*ebiten.Image{g.OcclusionImage},
		})
	}

	// Draw our shadows.
	g.FinalImage.Clear()
	g.FinalImage.DrawImage(g.WorldImage, nil)
	g.FinalImage.DrawRectShader(g.OcclusionImage.Bounds().Dx(), g.OcclusionImage.Bounds().Dy(), g.ShadowShader, &ebiten.DrawRectShaderOptions{
		Images: [4]*ebiten.Image{g.PolarMapImage},
	})

	// Draw all our image steps to different quadrants.
	opts := &ebiten.DrawImageOptions{}
	screen.DrawImage(g.WorldImage, opts)
	opts.GeoM.Translate(401, 0)
	screen.DrawImage(g.OcclusionImage, opts)
	opts.GeoM.Translate(-401, 401)
	opts.GeoM.Scale(0, 400)
	screen.DrawImage(g.PolarMapImage, opts)
	opts.GeoM.Reset()
	opts.GeoM.Translate(401, 401)
	screen.DrawImage(g.FinalImage, opts)

	// Draw some lines to show the quadrants.
	vector.StrokeLine(screen, 401, 0, 401, 801, 1, color.NRGBA{255, 0, 0, 255}, false)
	vector.StrokeLine(screen, 0, 401, 801, 401, 1, color.NRGBA{255, 0, 0, 255}, false)
}

func (g *Game) Layout(ow, oh int) (w, h int) {
	return ow, oh
}

func main() {
	g := Game{}

	if err := ebiten.RunGameWithOptions(&g, &ebiten.RunGameOptions{
		GraphicsLibrary: ebiten.GraphicsLibraryOpenGL,
	}); err != nil {
		panic(err)
	}
}
