import {Compile, Lint, Tokenize, AutoComplete} from 'gopherjs-loader!../compiler.go';

 self.addEventListener('message', function(event){
   const data = event.data;
   switch (data.type) {
     case "lint":
     postMessage({
       id: data.id,
       result: Lint(data.code),
     });
     case "compile":
     postMessage({
       id: data.id,
       result: Compile(data.code),
     });
     case "tokenize":
     postMessage({
       id: data.id,
       result: Tokenize(data.code),
     });
     case "autocomplete":
     postMessage({
       id: data.id,
       result: AutoComplete(data.code, data.options.line, data.options.column),
     });
   }
 });
