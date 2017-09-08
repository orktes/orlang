import {Compile, Lint, Tokenize} from 'gopherjs-loader!../compiler.go';

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
   }
 });
