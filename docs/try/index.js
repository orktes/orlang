import {Lint, Compile} from './lib/compiler';
import React from 'react';
import { render } from 'react-dom';
import MonacoEditor from 'react-monaco-editor';
import _ from 'lodash';

var defaultCode =
`
// type your code here (global functions print & int_to_str)

fn main() {
  print("Hello world")
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
      var fn = new Function('print', 'printInt', 'int_to_str', res);
      var output = [];
      fn(
        (str)=> {
          output.push(str);
        },
        (val)=> {
          output.push("" + val)
        },
        (val)=> {
          return "" + val;
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
            options={{fontSize: 20}}
          />
        </div>
        <div className="monaco-editor" style={{margin: 10, fontSize: 20, width: "100%", height: "30%", color: 'white', backgroundColor: 'black'}}>
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
            {this.state.printOutput.map((line, i)=> {
              return <div key={i}>{line}</div>;
            })}
        </div>
      </div>
    );
  }
}

render(
  <App />,
  document.getElementById('content')
);