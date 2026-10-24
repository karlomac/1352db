use serde_json::Value;

pub fn type_order(v: &Value) -> u8 {
    match v {
        Value::Null => 0,
        Value::Bool(_) => 1,
        Value::Number(_) => 2,
        Value::String(_) => 3,
        Value::Array(_) => 4,
        Value::Object(_) => 5,
    }
}

pub fn compare(a: &Value, b: &Value) -> i8 {
    let ta = type_order(a);
    let tb = type_order(b);
    if ta != tb {
        return if ta < tb { -1 } else { 1 };
    }
    match (a, b) {
        (Value::Null, Value::Null) => 0,
        (Value::Bool(x), Value::Bool(y)) => {
            if x == y { 0 } else if !x { -1 } else { 1 }
        }
        (Value::Number(x), Value::Number(y)) => {
            let xf = x.as_f64().unwrap_or(0.0);
            let yf = y.as_f64().unwrap_or(0.0);
            if xf < yf { -1 } else if xf > yf { 1 } else { 0 }
        }
        (Value::String(x), Value::String(y)) => {
            if x < y { -1 } else if x > y { 1 } else { 0 }
        }
        (Value::Array(x), Value::Array(y)) => {
            let n = x.len().min(y.len());
            for i in 0..n {
                let c = compare(&x[i], &y[i]);
                if c != 0 { return c; }
            }
            if x.len() < y.len() { -1 } else if x.len() > y.len() { 1 } else { 0 }
        }
        (Value::Object(x), Value::Object(y)) => {
            let mut xk: Vec<&String> = x.keys().collect();
            let mut yk: Vec<&String> = y.keys().collect();
            xk.sort();
            yk.sort();
            let n = xk.len().min(yk.len());
            for i in 0..n {
                if xk[i] != yk[i] {
                    return if xk[i] < yk[i] { -1 } else { 1 };
                }
                let c = compare(&x[xk[i]], &y[yk[i]]);
                if c != 0 { return c; }
            }
            if xk.len() < yk.len() { -1 } else if xk.len() > yk.len() { 1 } else { 0 }
        }
        _ => 0,
    }
}