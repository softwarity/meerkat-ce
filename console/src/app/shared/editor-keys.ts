import { Extension, Prec } from '@codemirror/state';
import { keymap } from '@codemirror/view';

// Ctrl+S / Cmd+S in an editor of this console does what its Save button does.
//
// Every editor here sits in a dialog or a drawer with a Save button, and the
// fingers of anyone who writes code press the shortcut before they look for
// the button: left to the browser, it opened "Save page as" over the console.
// Highest precedence, so no binding of basicSetup gets there first; and the key
// is always taken, even when saving is not possible right now, so the browser's
// dialog never comes back through the cases where it is not.
// Mod is Cmd on a Mac and Ctrl elsewhere; Ctrl is added for the Mac too, where
// a hand that came from another system still reaches for it.
export function saveKey(run: () => void): Extension {
  const save = () => {
    run();
    return true;
  };
  return Prec.highest(
    keymap.of([
      { key: 'Mod-s', preventDefault: true, run: save },
      { key: 'Ctrl-s', preventDefault: true, run: save },
    ]),
  );
}
