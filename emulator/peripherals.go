package emulator

// Virtual Display

type VirtualDisplayUpdate struct {
	RegionX int      `json:"region_x"`
	RegionY int      `json:"region_y"`
	Data    []uint32 `json:"data"`
}

func (s *VirtualDisplay) GetUpdates() []VirtualDisplayUpdate {
	s.dataMutex.Lock()
	defer s.dataMutex.Unlock()

	numRegionsPerRow := (s.width + 15) / 16

	updates := make([]VirtualDisplayUpdate, 0)
	for y := 0; y < s.height; y += 16 {
		for x := 0; x < s.width; x += 16 {
			if s.updateRegions[(y>>4)*numRegionsPerRow+(x>>4)] {
				outData := make([]uint32, 16*16)
				for ox := 0; ox < 16; ox++ {
					for oy := 0; oy < 16; oy++ {
						outData[oy*16+ox] = s.data[(y+oy)*s.width+(x+ox)]
					}
				}

				updates = append(updates, VirtualDisplayUpdate{
					RegionX: x,
					RegionY: y,
					Data:    outData,
				})
				s.updateRegions[(y>>4)*numRegionsPerRow+(x>>4)] = false
			}
		}
	}

	return updates
}

func (s *VirtualDisplay) GetEntireScreen() []VirtualDisplayUpdate {
	s.dataMutex.Lock()
	defer s.dataMutex.Unlock()

	numRegionsPerRow := (s.width + 15) / 16

	updates := make([]VirtualDisplayUpdate, 0)
	for y := 0; y < s.height; y += 16 {
		for x := 0; x < s.width; x += 16 {
			outData := make([]uint32, 16*16)
			for ox := 0; ox < 16; ox++ {
				for oy := 0; oy < 16; oy++ {
					outData[oy*16+ox] = s.data[(y+oy)*s.width+(x+ox)]
				}
			}

			updates = append(updates, VirtualDisplayUpdate{
				RegionX: x,
				RegionY: y,
				Data:    outData,
			})
			s.updateRegions[(y>>4)*numRegionsPerRow+(x>>4)] = false
		}
	}

	return updates
}

func (s *VirtualDisplay) getUpdateOffset(dataOffset uint32) int {
	// there are (s.width+15)/16 update regions per row
	// each update region is 16x16 pixels
	numRegionsPerRow := (s.width + 15) / 16
	dataX := (dataOffset % uint32(s.width)) / 16
	dataY := (dataOffset / uint32(s.width)) / 16

	return int(dataY)*numRegionsPerRow + int(dataX)
}

func (s *VirtualDisplay) drawFilledRectangle(color uint32) {
	// uses shape draw params. 0 is x, 1 is y, 2 is width, 3 is height
	x := s.shapeDrawParams[0]
	y := s.shapeDrawParams[1]
	width := s.shapeDrawParams[2]
	height := s.shapeDrawParams[3]

	s.dataMutex.Lock()
	defer s.dataMutex.Unlock()
	for oy := 0; oy < int(height); oy++ {
		for ox := 0; ox < int(width); ox++ {
			s.data[(y+uint32(oy))*uint32(s.width)+(x+uint32(ox))] = color
		}
	}
}
