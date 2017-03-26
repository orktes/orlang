OrlangView = require './orlang-view'
{CompositeDisposable} = require 'atom'

module.exports = Orlang =
  orlangView: null
  modalPanel: null
  subscriptions: null

  activate: (state) ->
    @orlangView = new OrlangView(state.orlangViewState)
    @modalPanel = atom.workspace.addModalPanel(item: @orlangView.getElement(), visible: false)

    # Events subscribed to in atom's system can be easily cleaned up with a CompositeDisposable
    @subscriptions = new CompositeDisposable

    # Register command that toggles this view
    @subscriptions.add atom.commands.add 'atom-workspace', 'orlang:toggle': => @toggle()

  deactivate: ->
    @modalPanel.destroy()
    @subscriptions.dispose()
    @orlangView.destroy()

  serialize: ->
    orlangViewState: @orlangView.serialize()

  toggle: ->
    console.log 'Orlang was toggled!'

    if @modalPanel.isVisible()
      @modalPanel.hide()
    else
      @modalPanel.show()
