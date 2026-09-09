import { describe, expect, test } from 'vitest'

import { buildKnowledgeTree, collectKnowledgeDocumentIds } from './tree'

describe('knowledge tree', () => {
  test('preserves nested directories and their documents', () => {
    const tree = buildKnowledgeTree([
      { id: 1, relative_path: 'k8s/k8s-clusters.md' },
      { id: 2, relative_path: 'storage/juicefs/mount.md' },
      { id: 3, relative_path: 'README.md' },
    ])

    expect(tree.children.map((node) => node.name)).toEqual([
      'k8s',
      'storage',
      'README.md',
    ])
    expect(tree.children[1].children[0].name).toBe('juicefs')
    expect(collectKnowledgeDocumentIds(tree.children[1])).toEqual([2])
  })
})
