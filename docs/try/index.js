import {Lint, Compile} from './lib/compiler';
import React from 'react';
import { render } from 'react-dom';
import MonacoEditor from 'react-monaco-editor';
import _ from 'lodash';

var defaultCode =
`// This macro uses build-in function print to print a list of expressions
macro print {
  ($a:expr) : (
    print($a)
    print("\\n")
  )

  ($a:expr, $( $b:expr ),*) : (
    print($a)
    $(
      print(" ")
      print($b)
    )*
    print("\\n")
  )
}

// Example of an interface that defines a reset function
interface Resetable {
  fn reset()
}

// Example of a struct
struct Point {
  var x = 0
  var y = 0

  // Example of an operator overload
  fn +(left:Point, right:Point) => Point {
    var newPoint = Point{}
    newPoint.x = left.x + right.x
    newPoint.y = left.y + right.y
    return newPoint
  }

  // Example of an operator overload
  fn -(left:Point, right:Point) => Point {
    var newPoint = Point{}
    newPoint.x = left.x - right.x
    newPoint.y = left.y - right.y
    return newPoint
  }

  // Implement Resetable interface
  fn reset() {
    this.x = 0
    this.y = 0
  }

  // Implement buildin stringer interface (argument type of the print function)
  fn toString() => string {
    return "{\\n" +
           "  x: " + this.x.toString() +
           ",\\n" +
           "  y: " + this.y.toString() +
           "\\n}"
  }
}

fn reset(resetable : Resetable) {
  resetable.reset()
}

fn main() {
  var pointA : Point
  pointA = Point{10, 10}

  // Type can be omitted when default value is used
  var pointB = Point{20, 30}

  // This is using the operator overload defined in the Point struct
  var combined = pointA + pointB

  // Use print macro to print the result
  print!("Hello world:", combined)

  // Example usage of an interface
  reset(combined)
  print!("Value after reset:", combined)
}
`;

class App extends React.Component {
  _printOutput = []

  state = {
    lintErrors: [],
    printOutput: [],
    code: defaultCode,
  }

  componentDidMount() {
    window.addEventListener('resize', ()=> {
      this._editor.layout();
    });
  }

  _printErrorsToConsole(errors) {
    this.setState({
      lintErrors: errors
    });
  }

  _clearConsole() {
    this.setState({
      lintErrors: [],
      printOutput: [],
    });
  }

  _compileCode = (code) => {
    Compile(code).then((res)=> {
      var fn = new Function('print', res);
      var output = [];
      fn(
        (str)=> {
          output.push(str && str.toString());
        }
      );
      this.setState({
        printOutput: output
      });
    });
  }

  _lintCode = _.debounce(() => {
    const code = this.state.code;
    Lint(code).then((res)=> {
      this._clearConsole();
      this._printErrorsToConsole(res);
      const monaco = this._monaco;
      const model = this._editor.getModel();
      let error = false;
      monaco.editor.setModelMarkers(model, 'Orlang', res.map((lintError)=> {
        if (!lintError.Warning) {
          error = true;
        }
        return {
          severity: lintError.Warning ? monaco.Severity.Warning : monaco.Severity.Error,
          code: null,
          source: null,
          startLineNumber: lintError.Position.Line + 1,
          startColumn: lintError.Position.Column + 1,
          startLineNumber: lintError.EndPosition.Line + 1,
          startColumn: lintError.EndPosition.Column + 1,
          message: lintError.Message,
        }
      }));

      if (!error) {
        this._compileCode(code);
      }

    }).catch((err)=> {
      console.log(err);
    })
  }, 1000)

  onChange = (newValue, e) => {
    this.setState({code: newValue});
    this._lintCode();
  }

  editorDidMount = (editor, monaco) => {
    this._editor = editor;
    this._monaco = monaco;
    this._lintCode(defaultCode);
  }

  render() {
    return (
      <div style={{position: 'absolute', top: 0, left: 0, right: 0, bottom: 0}}>
        <div style={{width: "100%", height: "70%"}}>
          <MonacoEditor
            theme="vs-dark"
            language="text"
            value={this.state.code}
            editorDidMount={this.editorDidMount}
            onChange={this.onChange}
            options={{fontSize: 14}}
          />
        </div>
        <div className="monaco-editor" style={{overflowY: 'scroll', margin: 10, fontSize: 14, width: "100%", height: "30%", color: 'white', backgroundColor: 'black'}}>
            {this.state.lintErrors.length > 0 ? <div>Lint errors:</div> : null}
            {this.state.lintErrors.map((error, i) => {
              return <div onClick={(e)=> {
                  const pos = {column: error.Position.Column + 1, lineNumber: error.Position.Line + 1};
                  this._editor.focus();
                  this._editor.setPosition(pos);
                  this._editor.revealPositionInCenter(pos);
                }} style={{color: error.Warning ? 'blue' : 'red' }} key={i}>{error.Position.Line + 1}:{error.Position.Column + 1}: {error.Message}</div>;
            })}
            <br />
            {this.state.printOutput.length > 0 ? <div>Output:</div> : null}
            <pre>
              {this.state.printOutput.join('')}
            </pre>
        </div>
      </div>
    );
  }
}

render(
  <App />,
  document.getElementById('content')
);
