var candidates = []
var stdoutLeak = []
var edit: = (ns [
    &completion:=(ns [
        &arg-completer=[&]
    ])
    &notify~={|msg|
        set stdoutLeak = [ $@stdoutLeak (to-string $msg) ]
    }
    &complex-candidate~={|val &display="" &code-suffix=""|
        set candidates = [ $@candidates [&value=$val &display=(to-string $display) &code-suffix=$code-suffix] ]
    }
])
eval %q
if (has-key $edit:completion:arg-completer %s) {
    var rawOutputs = [ ($edit:completion:arg-completer[%s] %s) ]
    for out $rawOutputs {
        set stdoutLeak = [ $@stdoutLeak (to-string $out) ]
    }
}
put [&candidates=$candidates &stdoutLeak=$stdoutLeak] | to-json
