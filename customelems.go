package wade

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	jq "github.com/gopherjs/jquery"
)

const (
	ModelIdAttr = "data-wade-modelid"
)

type CustomTag struct {
	meid  string //id of the model welement used to declare the tag content
	model interface{}
}

type CustagMan struct {
	elemModels []interface{}
	custags    map[string]*CustomTag
	tcontainer jq.JQuery
}

func newCustagMan(tcontainer jq.JQuery) *CustagMan {
	return &CustagMan{
		custags:    make(map[string]*CustomTag),
		elemModels: make([]interface{}, 0),
		tcontainer: tcontainer,
	}
}

func (tm *CustagMan) modelForElem(elem jq.JQuery) interface{} {
	mi := elem.Attr(ModelIdAttr)
	if mi == "" {
		panic("no modelId assigned for the element, something's wrong?")
	}
	modelId, err := strconv.Atoi(mi)
	if err != nil {
		panic("wrong format for internal element attribute " + ModelIdAttr + ", something's wrong?")
	}
	return tm.elemModels[modelId]
}

func (tm *CustagMan) RegisterNew(tagid string, model interface{}) {
	tagElem := tm.tcontainer.Find("#" + tagid)
	if tagElem.Length == 0 {
		panic(fmt.Sprintf("Welement with id #%v does not exist.", tagid))
	}
	if !tagElem.Is("welement") {
		panic(fmt.Sprintf("The element #%v to register new tag must be a welement.", tagid))
	}
	tm.custags[strings.ToUpper(tagid)] = &CustomTag{tagid, model}
}

func (tm *CustagMan) IsCustomElem(elem jq.JQuery) bool {
	_, ok := tm.custags[strings.ToUpper(elem.Prop("tagName").(string))]
	return ok
}

func (tm *CustagMan) prepare() {
	for _, tag := range tm.custags {
		mtype := reflect.TypeOf(tag.model)
		if mtype.Kind() != reflect.Struct {
			panic(fmt.Sprintf("Wrong type for the model of tag #%v, it must be a struct (non-pointer).", tag.meid))
		}
		tm.prepareTag(tag.meid, mtype)
	}
}

func (tm *CustagMan) prepareTag(tagid string, model reflect.Type) {
	tagElem := tm.tcontainer.Find("#" + tagid)
	publicAttrs := []string{}
	if attrs := tagElem.Attr("attributes"); attrs != "" {
		publicAttrs = strings.Split(attrs, " ")
		for _, attr := range publicAttrs {
			if _, ok := model.FieldByName(attr); !ok {
				panic(fmt.Sprintf(`Attribute "%v" is not available in the model for custom tag "%v".`, attr, tagid))
			}
		}
	}

	elems := tm.tcontainer.Find(tagid)
	elems.Each(func(idx int, elem jq.JQuery) {
		cptr := reflect.New(model)
		clone := cptr.Elem()
		for _, attr := range publicAttrs {
			if val := elem.Attr(AttrPrefix + attr); val != "" {
				field := clone.FieldByName(attr)
				var err error = nil
				var v interface{}
				ftype, _ := model.FieldByName(attr)
				kind := ftype.Type.Kind()
				switch kind {
				case reflect.Int:
					v, err = strconv.Atoi(val)
				case reflect.Uint:
					var m uint32
					var n uint64
					n, err = strconv.ParseUint(val, 10, 32)
					m = uint32(n)
					v = m
				case reflect.Float32:
					v, err = strconv.ParseFloat(val, 32)
				case reflect.Bool:
					v, err = strconv.ParseBool(val)
				case reflect.String:
					v = val
				default:
					if kind == reflect.Map {
						v = reflect.MakeMap(ftype.Type)
					}
					err = fmt.Errorf(`Unhandled type "%v", cannot use normal html to set the attribute "%v" of custom tag "%v".
consider using attribute binding instead.`, kind, attr, tagid)
				}

				if err != nil {
					panic(fmt.Sprintf(`Invalid value "%v" for attribute "%v" of custom tag "%v": type mismatch. Parse info: %v.`,
						val, attr, tagid, err))
				}

				field.Set(reflect.ValueOf(v))
			}
		}

		tm.elemModels = append(tm.elemModels, cptr.Interface())
		elem.SetAttr(ModelIdAttr, strconv.Itoa(len(tm.elemModels)-1))
	})
}