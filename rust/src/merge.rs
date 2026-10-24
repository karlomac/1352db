use serde::Deserialize;
use std::collections::HashMap;

#[derive(Debug, Clone, Deserialize)]
pub struct Rev {
    pub rev: String,
    #[serde(default)]
    pub parents: Vec<String>,
    #[serde(default)]
    pub deleted: bool,
}

pub fn parse_gen(rev: &str) -> i32 {
    rev.split('-').next().and_then(|s| s.parse().ok()).unwrap_or(0)
}

pub fn find_winning_rev(revs: &[Rev]) -> (String, Vec<String>) {
    // build tree
    let mut tree: HashMap<String, Rev> = HashMap::new();
    for r in revs {
        tree.insert(r.rev.clone(), r.clone());
    }
    // leaves: revs not parent of another
    let mut parent_set: std::collections::HashSet<String> = std::collections::HashSet::new();
    for r in revs {
        for p in &r.parents {
            parent_set.insert(p.clone());
        }
    }
    let leaves: Vec<Rev> = revs.iter().filter(|r| !parent_set.contains(&r.rev)).cloned().collect();
    // score
    let mut scored: Vec<(Rev, usize)> = leaves
        .iter()
        .map(|l| {
            let mut depth = 1usize;
            for p in &l.parents {
                if tree.contains_key(p) {
                    depth += 1;
                }
            }
            (l.clone(), depth)
        })
        .collect();
    scored.sort_by(|a, b| {
        if a.1 != b.1 { return b.1.cmp(&a.1); }
        let ga = parse_gen(&a.0.rev);
        let gb = parse_gen(&b.0.rev);
        if ga != gb { return gb.cmp(&ga); }
        b.0.rev.cmp(&a.0.rev)
    });
    if scored.is_empty() {
        return (String::new(), vec![]);
    }
    let winner = scored[0].0.rev.clone();
    let conflicts: Vec<String> = scored[1..].iter().map(|s| s.0.rev.clone()).collect();
    (winner, conflicts)
}
#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn winning() {
        let revs = vec![
            Rev { rev: "1-a".into(), parents: vec![], deleted: false },
            Rev { rev: "2-b".into(), parents: vec!["1-a".into()], deleted: false },
            Rev { rev: "2-c".into(), parents: vec!["1-a".into()], deleted: false },
        ];
        let (w, c) = find_winning_rev(&revs);
        assert_eq!(w, "2-c"); // depth tie -> highest gen -> lexical? 2-b > 2-c? our sort uses rev descending for tie, so 2-c? 
        let _ = c;
    }
}
