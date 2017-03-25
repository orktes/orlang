AtomView = require './atom-view'
{CompositeDisposable} = require 'atom'

module.exports = Atom =
  atomView: null
  modalPanel: null
  subscriptions: null

  activate: (state) ->
    @atomView = new AtomView(state.atomViewState)
    @modalPanel = atom.workspace.addModalPanel(item: @atomView.getElement(), visible: false)

    # Events subscribed to in atom's system can be easily cleaned up with a CompositeDisposable
    @subscriptions = new CompositeDisposable

    # Register command that toggles this view
    @subscriptions.add atom.commands.add 'atom-workspace', 'atom:toggle': => @toggle()

  deactivate: ->
    @modalPanel.destroy()
    @subscriptions.dispose()
    @atomView.destroy()

  serialize: ->
    atomViewState: @atomView.serialize()

  toggle: ->
    console.log 'Atom was toggled!'

    if @modalPanel.isVisible()
      @modalPanel.hide()
    else
      @modalPanel.show()
