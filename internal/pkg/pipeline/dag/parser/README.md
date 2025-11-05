# DAG Parser

The DAG Parser is the core component responsible for parsing DAG (Directed Acyclic Graph) expressions and converting them into an executable task dependency graph. It transforms human-readable pipeline expressions into optimized data structures for parallel execution.

## Purpose

The parser enables users to define complex data processing workflows using a simple, intuitive syntax similar to shell pipes but extended for parallel and branching operations. It handles the complexity of dependency resolution, task ordering, and graph optimization automatically.

## Architecture

The parser follows a traditional compiler design pattern with three main phases:

### 1. Tokenization (`tokenizer.go`)
Breaks down the input DAG expression into a stream of tokens:

**Token Types:**
- `IDENTIFIER`: Task names (e.g., `task1`, `extract_data`)
- `ARROW`: Flow operator (`>>`)
- `LBRACKET`: Left bracket (`[`)
- `RBRACKET`: Right bracket (`]`)
- `COMMA`: Tuple separator (`,`)
- `EOF`: End of input

**Example Tokenization:**
```
Input: "task1 >> [task2, task3]"
Tokens: [IDENTIFIER:task1, ARROW, LBRACKET, IDENTIFIER:task2, COMMA, IDENTIFIER:task3, RBRACKET, EOF]
```

### 2. Parsing (`parser.go`)
Uses recursive descent parsing to build an Abstract Syntax Tree (AST).

### 3. AST Building (`ast.go`)
Creates a tree (abstract syntax tree) structure representing the pipeline dependencies:

**AST Node Types:**
- `Ident`: Represents individual tasks
- `BinOp`: Represents binary operations (e.g., `>>`)
- `Tuple`: Represents grouped operations (e.g., `[a, b]`)

## Expression Language

### Syntax Reference

| Expression | Meaning | Result |
|------------|---------|---------|
| `task1` | Single task | task1 executes |
| `task1 >> task2` | Sequential | task1 then task2 |
| `[task1, task2]` | Parallel | task1 and task2 simultaneously |
| `task1 >> [task2, task3]` | Fan-out | task1 feeds both task2 and task3 |
| `[task1, task2] >> task3` | Fan-in | Both task1 and task2 feed task3 |

### Complex Examples

**Diamond Pattern:**
```yaml
dag: task1 >> [task2, task3] >> task4
```
Creates: task1 → {task2, task3} → task4

**Multi-level Branching:**
```yaml
dag: source >> [branch1 >> process1, branch2 >> process2] >> sink
```

**Multiple Sources:**
```yaml
dag: [source1, source2, source3] >> normalize >> output
```

## Parsing Process

### Phase 1: Lexical Analysis
```go
tokenizer := NewTokenizer(dagExpression)
tokens := tokenizer.TokenizeAll()
```

### Phase 2: Syntax Analysis
```go
parser := NewParser(tokens)
ast, err := parser.Parse()
```

### Phase 3: Dependency Resolution
```go
nodes, err := ast.ResolveLeft(taskRegistry)
```

## AST Structure

### Expression Interface
All AST nodes implement the `Expr` interface:
```go
type Expr interface {
    PostOrder() []string
    ResolveLeft(func(name string) task.Task) ([]*Node, error)
    ResolveRight(func(name string) task.Task) ([]*Node, error)
}
```

### Node Types

**Identifier Node:**
```go
type Ident struct {
    Name string
}
```
Represents a single task reference.

**Binary Operation Node:**
```go
type BinOp struct {
    Left  Expr
    Op    string  // Currently only ">>"
    Right Expr
}
```
Represents sequential operations with dependency relationships.

**Tuple Node:**
```go
type Tuple struct {
    Elements []Expr
}
```
Represents parallel operations that can execute simultaneously.

## Dependency Resolution

### Algorithm Overview
1. **Left Resolution**: Identifies source nodes (no dependencies)
2. **Right Resolution**: Identifies sink nodes (no dependents)  
3. **Connection Building**: Creates edges between dependent tasks
4. **Mux/Demux Insertion**: Adds coordination tasks for complex topologies

### Node Registry
Prevents duplicate task instances and ensures proper dependency tracking:
```go
var globalNodeRegistry map[string]*Node
```

### Connection Management
- Detects and prevents duplicate connections
- Handles fan-out patterns with Mux task insertion
- Handles fan-in patterns with Demux task insertion

## Error Handling

### Parse Errors
- **Syntax Errors**: Invalid token sequences (e.g., `task1 >> >>`)
- **Bracket Mismatches**: Unclosed parentheses (e.g., `(task1, task2`)
- **Empty Expressions**: Blank or whitespace-only DAG strings

### Resolution Errors
- **Missing Tasks**: References to undefined tasks
- **Circular Dependencies**: Cycles in the dependency graph
- **Invalid Operations**: Unsupported operators or structures

### Error Examples
```go
// Syntax error
dag: "task1 >> >> task2"
// Error: unexpected token '>>' at position 10

// Missing task
dag: "task1 >> nonexistent"
// Error: task not found: nonexistent

// Circular dependency
dag: "task1 >> task2 >> task1" 
// Error: circular dependency detected
```

## Performance Optimization

### AST Reuse
- Node registry prevents duplicate task creation
- Shared references reduce memory usage
- Faster resolution for complex graphs

### Lazy Evaluation
- Dependency resolution occurs only when needed
- PostOrder traversal is cached for efficiency
- Connection building is optimized for common patterns

## Testing and Validation

### Unit Tests
The parser includes comprehensive test coverage:
- Token recognition and classification
- Grammar rule validation  
- AST construction correctness
- Error condition handling


## Usage in Pipeline

### Configuration Integration
```yaml
tasks:
  # Task definitions...

dag: |
  extract >> [validation1, validation2] >> load
```

### Runtime Integration
```go
// Parse DAG expression
parser := NewParser(tokenizer.TokenizeAll())
ast, err := parser.Parse()

// Resolve dependencies
nodes, err := ast.ResolveLeft(taskRegistry)

// Execute DAG
dag := dag(nodes)
errors := dag.Run(wg, locker, channelSize)
```

## Debugging Support

### AST Visualization
```go
// Print AST structure
fmt.Println("PostOrder:", ast.PostOrder())

// Show dependencies
for _, node := range nodes {
    fmt.Printf("%s depends on: %v\n", node.Name, node.Upstream())
}
```

### Parse Tree Output
Enable verbose parsing to see token-by-token processing and AST construction steps.

## Limitations and Constraints

### Expression Complexity
- Maximum recommended nesting depth: 5 levels
- Fan-out width should be kept reasonable (< 20 branches)
- Very large expressions may impact parse time

### Syntax Restrictions
- Task names must be valid identifiers (alphanumeric + underscore)
- No support for conditional expressions
- No variable substitution in expressions

### Performance Considerations
- Parse time is O(n) where n is expression length
- Memory usage scales with AST depth and width
- Complex graphs may require optimization for large deployments
