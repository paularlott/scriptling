package object

// DeepCopy creates a deep copy of an object.
//
// Important limitations:
// - Circular references are NOT handled and will cause infinite recursion
// - NOT safe for concurrent modification: The caller must ensure the object
//   is not being modified during DeepCopy. This function captures slice
//   references but does not acquire locks on the source objects.
//
// This function is primarily used for cloning environments when spawning
// goroutines (e.g., async.run), where the source environment is not being
// concurrently modified by design.
func DeepCopy(obj Object) Object {
	switch v := obj.(type) {
	case *List:
		// Make a snapshot of the elements slice to avoid race conditions
		elements := v.Elements
		newElements := make([]Object, 0, len(elements))
		for _, elem := range elements {
			newElements = append(newElements, DeepCopy(elem))
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
		// Make a snapshot of the elements slice to avoid race conditions
		elements := v.Elements
		newElements := make([]Object, 0, len(elements))
		for _, elem := range elements {
			newElements = append(newElements, DeepCopy(elem))
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
