package prolog

// BuiltinRules contains Prolog rules that enable semantic reasoning
// beyond direct fact lookup. These are loaded automatically when the
// engine is created.
const BuiltinRules = `
% Transitive relatedness (2-hop via Prolog backtracking)
transitive(X, Y) :- related_to(X, Z), related_to(Z, Y), X \= Y.

% Same-domain: concepts sharing a taxonomy category
same_domain(X, Y) :- category(X, C), category(Y, C), X \= Y.

% Synonym equivalence through canonical forms
equivalent(X, Y) :- synonym(X, C), synonym(Y, C), X \= Y.

% Cross-domain bridge: concepts from different categories that co-occur
bridge(X, Y) :- category(X, C1), category(Y, C2), C1 \= C2, related_to(X, Y).

% Unified expansion: combines all reasoning modes for query expansion
expand_token(T, X) :- related_to(T, X).
expand_token(T, X) :- transitive(T, X).
expand_token(T, X) :- same_domain(T, X).
expand_token(T, X) :- equivalent(T, X).
expand_token(T, X) :- bridge(T, X).

% Composed expansion: find neighbors of T, then find THEIR domain siblings.
% This reaches concepts that share a category with a related concept,
% even when T itself has no category.
expand_token(T, X) :- related_to(T, Z), same_domain(Z, X), T \= X.
expand_token(T, X) :- related_to(T, Z), equivalent(Z, X), T \= X.

% === Typed expansion rules (powered by reltype classifier) ===
% These use typed edges (same_as, broader, narrower) inferred from
% distributional statistics. When typed edges exist, these rules
% enable directional query expansion.

% Synonym expansion: same_as is symmetric
expand_synonym(T, X) :- same_as(T, X).
expand_synonym(T, X) :- same_as(X, T).
expand_synonym(T, X) :- synonym(T, X).
expand_synonym(T, X) :- synonym(X, T).
expand_synonym(T, X) :- equivalent(T, X).

% Upward expansion: find broader (more general) concepts
expand_broader(T, X) :- broader(T, X).
expand_broader(T, X) :- broader(T, Z), broader(Z, X), T \= X.

% Downward expansion: find narrower (more specific) concepts
expand_narrower(T, X) :- narrower(T, X).
expand_narrower(T, X) :- narrower(T, Z), narrower(Z, X), T \= X.

% Typed expansion: combines typed edges with direction control.
% expand_typed/2 collects all typed expansions for use in search.
expand_typed(T, X) :- expand_synonym(T, X).
expand_typed(T, X) :- expand_broader(T, X).
expand_typed(T, X) :- expand_narrower(T, X).

% Also include untyped expansion for backwards compatibility
expand_typed(T, X) :- expand_token(T, X).
`
