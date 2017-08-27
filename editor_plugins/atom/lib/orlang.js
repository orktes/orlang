'use babel';

/* eslint-disable import/no-extraneous-dependencies, import/extensions */
import { CompositeDisposable } from 'atom';
/* eslint-enable import/no-extraneous-dependencies, import/extensions */

// Internal variables
let helpers = null;
let executablePath;

const lint = (editor, command) => {
  if (!helpers) {
    helpers = require('atom-linter');
  }

  const file = editor.getPath();
  const text = editor.getText();

  const args = ["lint", "--format=json", file]

  return helpers.exec(command, args, { stream: 'both' }).then((output) => {
    if (editor.getText() !== text) {
      // Editor contents changed, tell Linter not to update
      return null;
    }

    var errors = [];

    if (output.exitCode === 0) {
      const resp = JSON.parse(output.stdout);
      if (resp[0] && resp[0].Errors) {
        errors = resp[0].Errors.map((err)=> {
          let pos;
          if (err.EndPosition) {
            pos = [[err.Position.Line, err.Position.Column], [err.EndPosition.Line, err.EndPosition.Column]];
          } else {
            pos = helpers.generateRange(editor, err.Position.Line, err.Position.Column);
          }
          return {
            range: pos,
            type: err.Warning ? 'Warning' : 'Error',
            text: err.Message,
            filePath: file,
          }
        });
      }
    }
    return errors;
  });
};

export default {
  activate() {
    require('atom-package-deps').install('orlang');

    const linterName = 'orlang';

    this.subscriptions = new CompositeDisposable();

    this.subscriptions.add(
      atom.config.observe(`${linterName}.executablePath`, (value) => {
        executablePath = value;
      }),
    );
  },

  deactivate() {
    this.subscriptions.dispose();
  },

  provideLinter() {
    return {
      grammarScopes: ['source.orlang'],
      scope: 'file',
      lintOnFly: false,
      name: 'orlang',
      lint: editor => lint(editor, executablePath),
    };
  },
};
