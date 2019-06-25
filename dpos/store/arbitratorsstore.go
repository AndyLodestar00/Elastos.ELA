package store

import (
	"errors"
	"sort"

	"github.com/elastos/Elastos.ELA/common/log"
	"github.com/elastos/Elastos.ELA/dpos/state"
)

func (s *DposStore) GetHeightsDesc() ([]uint32, error) {
	heights, err := s.getHeights()
	if err != nil {
		return nil, err
	}
	sort.Slice(heights, func(i, j int) bool {
		return heights[i] > heights[j]
	})
	return heights, err
}

func (s *DposStore) GetCheckPoint(height uint32) (*state.CheckPoint, error) {
	heights, err := s.GetHeightsDesc()
	if err != nil {
		return nil, err
	}

	for i := 0; i < len(heights); i++ {
		if height >= heights[i]+state.CheckPointInterval {
			if !s.chainParams.CheckPointNoFlatFile {
				return s.getFlatCheckPoint(heights[i])
			} else {
				return s.getSingleCheckPoint(heights[i])
			}
		}
	}
	return nil, errors.New("can't find check point")
}

func (s *DposStore) SaveArbitersState(point *state.CheckPoint) (err error) {
	batch := s.db.NewBatch()

	if err = s.appendHeights(batch, point.Height); err != nil {
		log.Warn("[SaveArbitersState] appendHeights err: ", err)
		return
	}

	if !s.chainParams.CheckPointNoFlatFile {
		if err = s.saveFlatCheckPoint(point); err != nil {
			log.Warn("[SaveArbitersState] saveFlatCheckPoint err: ", err)
			return
		}
	} else {
		if err = s.persistSingleCheckPoint(batch, point.Height, point);
			err != nil {
			log.Warn("[SaveArbitersState] persistSingleCheckPoint err: ", err)
			return
		}
	}

	if err = batch.Commit(); err != nil {
		log.Warn("[SaveArbitersState] batch commit err: ", err)
	}
	return
}
