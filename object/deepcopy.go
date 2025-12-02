package object

// DeepCopy creates a deep copy of an object
// Note: Circular references are not handled and will cause infinite recursion
func DeepCopy(obj Object) Object {
	switch v := obj.(type) {
	case *List:
		newElements := make([]Object, len(v.Elements))
		for i, elem := range v.Elements {
			newElements[i] = DeepCopy(elem)
		}
		return &List{Elements: newElements}

	case *Dict:
		newPairs := make(map[string]DictPair, len(v.Pairs))
		for k, pair := range v.Pairs {
			newPairs[k] = DictPair{
				Key:   DeepCopy(pair.Key),
				Value: DeepCopy(pair.Value),
			}
		}
		return &Dict{Pairs: newPairs}

	case *Tuple:
		newElements := make([]Object, len(v.Elements))
		for i, elem := range v.Elements {
			newElements[i] = DeepCopy(elem)
		}
		return &Tuple{Elements: newElements}

	case *Integer:
		return NewInteger(v.Value)

	case *Float:
		return &Float{Value: v.Value}

	case *String:
		return &String{Value: v.Value}

	case *Boolean:
		if v.Value {
			return &Boolean{Value: true}
		}
		return &Boolean{Value: false}

	case *Null:
		return &Null{}

	case *Function:
		// Clone function with new environment
		// Note: We can't clone the environment here easily without creating a cycle if we're not careful
		// But Environment is in this package, so we can add a Clone method to Environment!
		return &Function{
			Name:          v.Name,
			Parameters:    v.Parameters,
			DefaultValues: v.DefaultValues,
			Variadic:      v.Variadic,
			Body:          v.Body,
			Env:           v.Env.Clone(),
		}

	case *LambdaFunction:
		return &LambdaFunction{
			Parameters:    v.Parameters,
			DefaultValues: v.DefaultValues,
			Variadic:      v.Variadic,
			Body:          v.Body,
			Env:           v.Env.Clone(),
		}

	default:
		// For other types, return the same object
		return obj
	}
}
