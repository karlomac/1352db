// Rust core for pdb: collation + merge, WASM-ready.

pub mod collate;
pub mod merge;

pub use collate::compare as collate_compare;
pub use merge::find_winning_rev as winning_rev;