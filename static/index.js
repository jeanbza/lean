const g = new dagreD3.graphlib.Graph().setGraph({})

g.setNode('loading', { label: 'loading' })

const svg = d3.select('svg'), inner = svg.select('g')

// Set up zoom support
const zoom = d3.zoom().on('zoom', function() {
  inner.attr('transform', d3.event.transform)
})
svg.call(zoom)

// Create the renderer
const render = new dagreD3.render()

// Run the renderer. This is what draws the final graph.
render(inner, g)

// Center the graph
const initialScale = 0.4
svg.call(zoom.transform, d3.zoomIdentity.translate(20, 0).scale(initialScale))

svg.attr('height', g.graph().height * initialScale + 40)

const bytesInMb = 1000000
const prettifySize = sizeBytes => {
  if (sizeBytes > 0) {
    const sizeMb = Math.floor(sizeBytes/bytesInMb)
    return `${sizeMb > 0 ? sizeMb : 1}mb`
  }
  return '(unknown size)'
}

const redrawGraph = graph => {
  // Remove initial node.
  g.removeNode('loading')

  // Remove all edges not in graph.
  g.edges().forEach(e => {
    if (graph[e.v] == undefined) {
      g.removeEdge(e.v, e.w)
      return
    }
    if (graph[e.v][e.w] == undefined) {
      g.removeEdge(e.v, e.w)
      return
    }
  })

  // Remove all edges not in graph.
  graphNodes = {}
  Object.entries(graph).forEach(entry => {
    const from = entry[0]
    const tos = entry[1]
    for (const to in tos) {
      graphNodes[from] = true
      graphNodes[to] = true
    }
  })
  g.nodes().forEach(n => {
    if (graphNodes[n] == undefined) {
      g.removeNode(n)
      return
    }
  })

  // Draw new graph.
  Object.entries(graph).forEach(entry => {
    const from = entry[0]
    const tos = entry[1]
    for (const to in tos) {
      const fromSize = prettifySize(tos[to].From.SizeBytes)
      const toSize = prettifySize(tos[to].To.SizeBytes)

      if (!g.hasNode(from) && !g.hasNode(to)) {
        g.setNode(from, {label: `${from}\n${fromSize}`})
        g.setNode(to, {label: `${to}\n${toSize}`})
        g.setEdge(from, to, {})
      } else if (!g.hasNode(from)) {
        g.setNode(from, {label: `${from}\n${fromSize}`})
        g.setEdge(from, to, {})
      } else if (!g.hasNode(to)) {
        g.setNode(to, {label: `${to}\n${toSize}`})
        g.setEdge(from, to, {})
      } else if (!g.hasEdge(from, to)) {
        g.setEdge(from, to, {})
      }
    }
  })

  // Render.
  render(inner, g)

  // Add hovers.
  d3.select('svg')
    .selectAll('path')
    .on('mouseover', function(e) { // Must be a func to have correct 'this' scope.
      focusInEdge(e.v, e.w)
    })
    .on('mouseout', function(e) { // Must be a func to have correct 'this' scope.
      focusOutEdge(e.v, e.w)
    })
}

const drawList = (id, entries, clickMethod) => {
  // Remove existing list.
  const el = document.getElementById(id)
  el.innerHTML = ''

  Object.entries(entries).forEach(entry => {
    const from = entry[0]
    const tos = entry[1]
    for (const to in tos) {
      const toSize = prettifySize(tos[to].To.SizeBytes)

      // Create a new list item.
      const newEdgeRow = document.createElement('div')

      // Add text.
      const rowText = document.createElement('div')
      rowText.innerHTML = `${from} -> ${to}`
      rowText.className = 'edge'
      newEdgeRow.appendChild(rowText)

      // Add size.
      const sizeText = document.createElement('div')
      sizeText.innerHTML = `${toSize}`
      sizeText.className = 'size'
      newEdgeRow.appendChild(sizeText)
  
      // Add button.
      const rowButton = document.createElement('button')
      rowButton.type = 'button'
      if (clickMethod == 'POST') {
        rowButton.innerHTML = 'Return'
      } else {
        rowButton.innerHTML = 'Remove'
      }
      rowButton.className = 'right'
      rowButton.onclick = _ => {
        fetch('/edge', {method: clickMethod, body: JSON.stringify({'from': from, 'to': to})}).then(resp => {
          resp.json().then(both => {
            redrawGraph(both['graph'])
            redrawEdgelist(both['graph'])
            redrawShoppingCart(both['shoppingCart'])
          })
        }).catch(err => console.error(err))
      }
      newEdgeRow.appendChild(rowButton)
  
      // Give the list item properties.
      newEdgeRow.id = `${id}-${from}${to}`
      newEdgeRow.className = 'edgeRow'
      newEdgeRow.dataset.from = from
      newEdgeRow.dataset.to = to

      // Give the list item an on-hover effect.
      newEdgeRow.onmouseover = _ => focusInEdge(from, to)
      newEdgeRow.onmouseout = _ => focusOutEdge(from, to)
      el.appendChild(newEdgeRow)
    }
  })
}

const focusInEdge = (from, to) => {
  // Colour edgerow in list below.
  document.getElementById(`edgeList-${from}${to}`).style.backgroundColor = 'red'
  document.getElementById(`edgeList-${from}${to}`).style.fontWeight = 'bold'

  // Colour edge.
  d3.select('svg')
    .selectAll('path')
    .filter(svgE => svgE.v == from && svgE.w == to)
    .each(function() {
      d3.select(this).style('stroke', 'red')
      d3.select(this).style('stroke-width', '5px')
    })

  fetch('/hypotheticalCut', {method: 'POST', body: JSON.stringify({'from': from, 'to': to})}).then(resp => {
    resp.json().then(respj => {
      const cutEdges = respj['edges']
      const cutVertices = respj['vertices']

      Object.entries(cutEdges).forEach(entry => {
        const from = entry[0]
        const tos = entry[1]
        for (const to in tos) {
          // Colour edgerow in list below.
          document.getElementById(`edgeList-${from}${to}`).style.backgroundColor = 'red'

          // Colour edge.
          d3.select('svg')
            .selectAll('path')
            .filter(svgE => svgE.v == from && svgE.w == to)
            .each(function() {
              d3.select(this).style('stroke', 'red')
            })
        }
      })

      // Colour vertex.
      Object.entries(cutVertices).forEach(varr => {
        const v = varr[1]
        d3.select('svg')
          .selectAll('tspan')
          .filter(spanText => spanText == v)
          .each(function() {
            const tspan = this
            d3.select(tspan).style('stroke', 'red')
            const text = tspan.parentNode
            const g1 = text.parentNode
            const g2 = g1.parentNode
            const g3 = g2.parentNode
            const rect = d3.select(g3).select('rect')
            rect.style('stroke', 'red')
          })
      })
    })
  })
}

const focusOutEdge = _ => {
  // Reset edgerow in list below.
  Array.from(document.getElementsByClassName('edgeRow')).forEach(e => {
    e.style.backgroundColor = 'transparent'
    e.style.fontWeight = 'normal'
  })
  
  // Reset vertices.
  d3.select('svg')
    .selectAll('rect')
    .each(function() {
      d3.select(this).style('stroke', 'black')
    })
  d3.select('svg')
    .selectAll('tspan')
    .each(function() {
      d3.select(this).style('stroke', 'black')
    })

  // Reset edges.
  d3.select('svg')
    .selectAll('path')
    .each(function() {
      d3.select(this).style('stroke', 'black')
      d3.select(this).style('stroke-width', '1.5px')
    })
}

const redrawEdgelist = graph => {
  drawList('edgeList', graph, 'DELETE')
}

const redrawShoppingCart = shoppingCart => {
  drawList('shoppingCart', shoppingCart, 'POST')
}

document.getElementById('reset').onclick = _ => {
  fetch('/reset').then(resp => {
    resp.json().then(both => {
      redrawGraph(both['graph'])
      redrawEdgelist(both['graph'])
      redrawShoppingCart(both['shoppingCart'])
    })
  }).catch(err => console.error(err))
}

fetch('/graph').then(resp => {
  resp.json().then(graph => {
    redrawGraph(graph)
    redrawEdgelist(graph)
  })
}).catch(err => console.error(err))

fetch('/shoppingCart').then(resp => {
  resp.json().then(shoppingCart => {
    redrawShoppingCart(shoppingCart)
  })
}).catch(err => console.error(err))
