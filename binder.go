package air

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"io/ioutil"
	"mime"
	"reflect"

	"github.com/BurntSushi/toml"
	"github.com/golang/protobuf/proto"
	"github.com/vmihailenco/msgpack"
)

// binder is a binder that binds request based on the MIME types.
type binder struct{}

// theBinder is the singleton of the `binder`.
var theBinder = &binder{}

// bind binds the r into the v.
func (b *binder) bind(v interface{}, r *Request) error {
	if r.Method == "GET" {
		return b.bindParams(v, r.Params())
	} else if r.Body == nil {
		return errors.New("request body cannot be empty")
	}

	mt, _, err := mime.ParseMediaType(r.Header("content-type").Value())
	if err != nil {
		return err
	}

	switch mt {
	case "application/json":
		err = json.NewDecoder(r.Body).Decode(v)
	case "application/xml":
		err = xml.NewDecoder(r.Body).Decode(v)
	case "application/msgpack", "application/x-msgpack":
		err = msgpack.NewDecoder(r.Body).Decode(v)
	case "application/protobuf", "application/x-protobuf":
		var b []byte
		if b, err = ioutil.ReadAll(r.Body); err == nil {
			err = proto.Unmarshal(b, v.(proto.Message))
		}
	case "application/toml", "application/x-toml":
		_, err = toml.DecodeReader(r.Body, v)
	case "application/x-www-form-urlencoded", "multipart/form-data":
		err = b.bindParams(v, r.Params())
	default:
		r.response.Status = 415
		return errors.New("unsupported media type")
	}

	if err != nil {
		return err
	}

	return nil
}

// bindParams binds the params into the v.
func (b *binder) bindParams(v interface{}, params []*RequestParam) error {
	typ := reflect.TypeOf(v).Elem()
	if typ.Kind() != reflect.Struct {
		return errors.New("binding element must be a struct")
	}

	val := reflect.ValueOf(v).Elem()
	for i := 0; i < typ.NumField(); i++ {
		vf := val.Field(i)
		if !vf.CanSet() {
			continue
		}

		vfk := vf.Kind()
		if vfk == reflect.Struct {
			err := b.bindParams(vf.Addr().Interface(), params)
			if err != nil {
				return err
			}

			continue
		}

		tf := typ.Field(i)

		var pv *RequestParamValue
		for _, p := range params {
			if p.Name == tf.Name {
				pv = p.Value()
				break
			}
		}

		if pv == nil {
			continue
		}

		switch tf.Type.Kind() {
		case reflect.Bool:
			b, _ := pv.Bool()
			vf.SetBool(b)
		case reflect.Int,
			reflect.Int8,
			reflect.Int16,
			reflect.Int32,
			reflect.Int64:
			i64, _ := pv.Int64()
			vf.SetInt(i64)
		case reflect.Uint,
			reflect.Uint8,
			reflect.Uint16,
			reflect.Uint32,
			reflect.Uint64:
			ui64, _ := pv.Uint64()
			vf.SetUint(ui64)
		case reflect.Float32, reflect.Float64:
			f64, _ := pv.Float64()
			vf.SetFloat(f64)
		case reflect.String:
			vf.SetString(pv.String())
		default:
			return errors.New("unknown type")
		}
	}

	return nil
}
