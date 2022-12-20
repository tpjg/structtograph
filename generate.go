package structtograph

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"reflect"
	"strings"
)

type Dot interface {
	AddStruct(i interface{}, flatten []string, opts ...Opts) error
	Connect(i1 interface{}, n1 string, i2 interface{}, n2 string, label ...string) error
	Output(w io.Writer) error
	OutputPng(fn string) error
}

type Opts struct {
	Rank     int
	NoFields bool
}

func Rank(r int) Opts {
	return Opts{Rank: r}
}

var ErrNotStruct = errors.New("not a struct type")

var maxdepth = 5 // maximum level of nesting flattened structs

type dot struct {
	directed bool
	structs  *bytes.Buffer
	conns    *bytes.Buffer
}

func NewDot(directed bool) Dot {
	return &dot{directed: directed, structs: new(bytes.Buffer), conns: new(bytes.Buffer)}
}

func (d *dot) Output(w io.Writer) error {
	if d.directed {
		fmt.Fprintf(w, "digraph ")
	} else {
		fmt.Fprintf(w, "graph ")
	}
	fmt.Fprintf(w, ` recordmapping {
	rankdir = "LR";
	nodesep=0.9;
	//compound=true;
	newrank=true;
	ranksep=0.9;

	fontname="Open Sans"
	node [fontname="Open Sans"]
	edge [fontname="Open Sans"]		
	node [fontsize = "16"];
	edge [fontsize = "12"];

`)
	_, err := w.Write(d.structs.Bytes())
	if err != nil {
		return err
	}
	fmt.Fprintln(w)
	_, err = w.Write(d.conns.Bytes())
	fmt.Fprintf(w, "\n}\n")
	return err
}

func (d *dot) OutputPng(fn string) error {
	fn = strings.TrimSuffix(fn, ".png")

	out, err := os.Create(fn + ".dot")
	if err != nil {
		return fmt.Errorf("error creating dot file: %w", err)
	}
	err = d.Output(out)
	if err != nil {
		return fmt.Errorf("error writing dot file: %w", err)
	}
	out.Close()

	cmd := exec.Command("dot", "-Tpng", "-o"+fn+".png", fn+".dot")
	cmd.Stderr, cmd.Stdout = os.Stderr, os.Stdout
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("error running dot: %w", err)
	}
	return nil
}

func (d *dot) AddStruct(i interface{}, flatten []string, opts ...Opts) error {
	if v := reflect.ValueOf(i); v.Kind() == reflect.Pointer || v.Kind() == reflect.Slice || v.Kind() == reflect.Array {
		i = v.Elem()
	}
	it := reflect.TypeOf(i)

	if strings.HasPrefix(reflect.ValueOf(i).String(), "reflect.") {
		return nil
	}

	if it.Kind() == reflect.Struct {
		fmt.Fprintf(d.structs, "subgraph \"cluster_%s\" {\n", it)
		fmt.Fprintf(d.structs, "  label = < <B>%s</B> >\n", it)
		fmt.Fprintf(d.structs, "  color = transparent\n")
		if len(opts) == 1 {
			fmt.Fprintf(d.structs, "  rank = %d\n", opts[0].Rank)
		}
		fmt.Fprintln(d.structs)
		fmt.Fprintf(d.structs, "\"%s\" [\n", it)
		if len(opts) == 1 && opts[0].NoFields {
			fmt.Fprintf(d.structs, "  label = \"%v\" \n", d.summaryStruct(it))
		} else {
			fmt.Fprintf(d.structs, "  label = \"%v\" \n", d.labelStruct(it, []string{}, flatten))
		}
		fmt.Fprintf(d.structs, "  shape = \"record\"\n")
		fmt.Fprintf(d.structs, "]\n}\n")
	}

	return ErrNotStruct
}

func (d *dot) summaryStruct(it reflect.Type) string {
	return fmt.Sprintf("<fields> %d ...", it.NumField())
}

func (d *dot) labelStruct(it reflect.Type, depth []string, flatten []string) string {
	if len(depth) > maxdepth {
		return ""
	}

	flattenMap := make(map[string]bool, len(flatten))
	for _, s := range flatten {
		flattenMap[s] = true
	}

	s := ""
	for i := 0; i < it.NumField(); i++ {
		field := it.Field(i)
		if strings.HasPrefix(field.Name, "XXX_") {
			continue
		}
		ft := field.Type
		if ft.Kind() == reflect.Pointer || ft.Kind() == reflect.Slice || ft.Kind() == reflect.Array {
			ft = ft.Elem()
		}
		if ft.Kind() == reflect.Struct {
			if _, ok := flattenMap[field.Name]; ok {
				flat := d.labelStruct(ft, append(depth, field.Name), flatten)
				if flat != "" {
					flat = "|" + flat
				}
				s += fmt.Sprintf("{<%v> %v %v }|", strings.Join(append(depth, field.Name), "_"), field.Name, flat)
				continue
			}
		}
		s += fmt.Sprintf("<%v> %v|", strings.Join(append(depth, field.Name), "_"), field.Name)
	}

	return strings.TrimSuffix(s, "|")
}

func (d *dot) Connect(i1 interface{}, n1 string, i2 interface{}, n2 string, label ...string) error {
	if v := reflect.ValueOf(i1); v.Kind() == reflect.Pointer || v.Kind() == reflect.Slice || v.Kind() == reflect.Array {
		i1 = v.Elem()
	}
	if v := reflect.ValueOf(i2); v.Kind() == reflect.Pointer || v.Kind() == reflect.Slice || v.Kind() == reflect.Array {
		i2 = v.Elem()
	}
	it1 := reflect.TypeOf(i1)
	it2 := reflect.TypeOf(i2)

	c1 := `"` + it1.String() + `"`
	c2 := `"` + it2.String() + `"`
	if it1.Kind() == reflect.Struct && n1 != "" {
		c1 += ":" + n1
	}
	if it2.Kind() == reflect.Struct && n2 != "" {
		c2 += ":" + n2
	}

	connector := "--"
	if d.directed {
		connector = "->"
	}

	if len(label) == 1 {
		c2 += fmt.Sprintf(" [ label = \"%s\" ]", label[0])
	}
	fmt.Fprintf(d.conns, "%s %s %s;\n", c1, connector, c2)
	return nil
}
