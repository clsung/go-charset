package charset

import (
	"bytes"
	"encoding/binary"
	"sort"
	"unicode/utf8"
)

func init() {
	registerClass("cp949", fromCp949, toCp949)
}

// code pair for a Korean chracter
type cp949Code struct {
	native  uint16 // cp949
	unicode rune   // ucs4
}

// lookup table for translator
type cp949Table []cp949Code

func (t cp949Table) Len() int {
	return len(t)
}

func (t cp949Table) Swap(i, j int) {
	t[i], t[j] = t[j], t[i]
}

// instance type to sort the lookup table by native code for from-translator
type cp949TableSortByNative struct{ cp949Table }

func (t cp949TableSortByNative) Less(i, j int) bool {
	return t.cp949Table[i].native < t.cp949Table[j].native
}

// instance type to sort the lookup table by unicode for to-translator
type cp949TableSortByUnicode struct{ cp949Table }

func (t cp949TableSortByUnicode) Less(i, j int) bool {
	return t.cp949Table[i].unicode < t.cp949Table[j].unicode
}

// use same struct to from-translator and to-translator.
// each translators use sort.Search() to find corresponding code.
// And, the lookup table is sorted by a filed which have
// same encoding of input data.
type translateCp949 struct {
	table   cp949Table // lookup table
	scratch []byte     // buffer for output
}

// from cp949 to unicode translator
type translateFromCp949 translateCp949

func (p *translateFromCp949) Translate(data []byte, eof bool) (int, []byte, error) {
	p.scratch = p.scratch[:0]
	c := 0
	for len(data) > 0 {
		if data[0]&0x80 == 0 {
			p.scratch = append(p.scratch, data[0])
			data = data[1:]
			c += 1
			continue
		}

		n := uint16(data[0])<<8 | uint16(data[1])
		fi := sort.Search(len(p.table), func(i int) bool {
			if n <= p.table[i].native {
				return true
			}
			return false
		})

		f := p.table[fi]
		if n == f.native {
			p.scratch = appendRune(p.scratch, f.unicode)
		} else {
			p.scratch = appendRune(p.scratch, utf8.RuneError)
		}
		data = data[2:]
		c += 2
	}
	return c, p.scratch, nil
}

// from unicode to cp949 translator
type translateToCp949 translateCp949

func (p *translateToCp949) Translate(data []byte, eof bool) (int, []byte, error) {
	p.scratch = p.scratch[:0]
	c := 0
	for len(data) > 0 {
		if data[0]&0x80 == 0 {
			p.scratch = append(p.scratch, data[0])
			data = data[1:]
			c += 1
			continue
		}

		r, s := utf8.DecodeRune(data)
		fi := sort.Search(len(p.table), func(i int) bool {
			if r <= p.table[i].unicode {
				return true
			}
			return false
		})

		f := p.table[fi]
		if r == f.unicode {
			p.scratch = append(p.scratch,
				byte(f.native>>8), byte(f.native&0xff))
		} else {
			p.scratch = append(p.scratch, '?')
		}

		data = data[s:]
		c += s
	}
	return c, p.scratch, nil
}

// load cp949.dat to cp949Table
func loadCp949Table() (cp949Table, error) {
	dat, err := readFile("cp949.dat")
	buf := bytes.NewReader(dat)

	// read info header
	var datInfo struct {
		CodeCnt, ChunkCnt uint16
	}
	if err = binary.Read(buf, binary.BigEndian, &datInfo); err != nil {
		return nil, err
	}

	// read code chunks to table
	table := make(cp949Table, datInfo.CodeCnt)
	table = table[:0]
	var chunk struct {
		Code, Len uint16
	}
	for i := uint16(0); i < datInfo.ChunkCnt; i++ {
		if err = binary.Read(buf, binary.BigEndian, &chunk); err != nil {
			return nil, err
		}

		line := make([]byte, chunk.Len)
		if n, err := buf.Read(line); n != int(chunk.Len) || err != nil {
			return nil, err
		}

		for _, u := range string(line) {
			table = append(table,
				cp949Code{native: chunk.Code, unicode: u})
			chunk.Code += 1
		}
	}

	return table, nil
}

// factory to create translateFromCp949
func fromCp949(arg string) (Translator, error) {
	type cp949KeyFrom bool
	table, err := cache(cp949KeyFrom(true), func() (interface{}, error) {
		t, err := loadCp949Table()
		if err != nil {
			return nil, err
		}
		if !sort.IsSorted(cp949TableSortByNative{t}) {
			panic("cp949.dat is not sorted by native code!")
		}
		return t, nil
	})
	if err != nil {
		return nil, err
	}
	return &translateFromCp949{table: table.(cp949Table)}, nil
}

// factory to create translateToCp949
func toCp949(arg string) (Translator, error) {
	type cp949KeyTo bool
	table, err := cache(cp949KeyTo(true), func() (interface{}, error) {
		t, err := loadCp949Table()
		if err != nil {
			return nil, err
		}
		sort.Sort(cp949TableSortByUnicode{t})
		return t, nil
	})
	if err != nil {
		return nil, err
	}
	return &translateToCp949{table: table.(cp949Table)}, nil
}
