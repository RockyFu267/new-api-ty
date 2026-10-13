export type KnowledgePathDocument = {
  id: number
  relative_path: string
}

export type KnowledgeTreeNode<T extends KnowledgePathDocument> = {
  kind: 'directory' | 'file'
  name: string
  path: string
  children: KnowledgeTreeNode<T>[]
  document?: T
}

export function buildKnowledgeTree<T extends KnowledgePathDocument>(
  documents: T[],
  directoryPaths: string[] = []
): KnowledgeTreeNode<T> {
  const root: KnowledgeTreeNode<T> = {
    kind: 'directory',
    name: 'knowledge-base',
    path: '',
    children: [],
  }

  for (const document of documents) {
    const segments = document.relative_path
      .replaceAll('\\', '/')
      .split('/')
      .filter(Boolean)
    let parent = root

    segments.forEach((segment, index) => {
      const path = segments.slice(0, index + 1).join('/')
      const isFile = index === segments.length - 1
      if (isFile) {
        parent.children.push({
          kind: 'file',
          name: segment,
          path,
          children: [],
          document,
        })
        return
      }

      let directory = parent.children.find(
        (node) => node.kind === 'directory' && node.name === segment
      )
      if (!directory) {
        directory = {
          kind: 'directory',
          name: segment,
          path,
          children: [],
        }
        parent.children.push(directory)
      }
      parent = directory
    })
  }

  for (const directoryPath of directoryPaths) {
    const segments = directoryPath.replaceAll('\\', '/').split('/').filter(Boolean)
    let parent = root
    segments.forEach((segment, index) => {
      const path = segments.slice(0, index + 1).join('/')
      let directory = parent.children.find(
        (node) => node.kind === 'directory' && node.name === segment
      )
      if (!directory) {
        directory = { kind: 'directory', name: segment, path, children: [] }
        parent.children.push(directory)
      }
      parent = directory
    })
  }

  sortTree(root)
  return root
}

function sortTree<T extends KnowledgePathDocument>(node: KnowledgeTreeNode<T>) {
  node.children.sort((left, right) => {
    if (left.kind !== right.kind) return left.kind === 'directory' ? -1 : 1
    return left.name.localeCompare(right.name)
  })
  node.children.forEach(sortTree)
}

export function collectKnowledgeDocumentIds<T extends KnowledgePathDocument>(
  node: KnowledgeTreeNode<T>
): number[] {
  if (node.document) return [node.document.id]
  return node.children.flatMap(collectKnowledgeDocumentIds)
}
