import { describe, expect, it } from 'vitest'
import { allTags, hasAllTags, matchesQuery } from './listFilter'

describe('matchesQuery', () => {
  it('EmptyQuery_MatchesEverything', () => {
    expect(matchesQuery('  ', ['x'])).toBe(true)
  })
  it('IgnoresCase', () => {
    expect(matchesQuery('GLOBEX', ['globex srl'])).toBe(true)
  })
  it('AllWordsMustMatchAcrossFields', () => {
    expect(matchesQuery('globex go', ['Globex', undefined, 'built in Go'])).toBe(true)
    expect(matchesQuery('globex rust', ['Globex', 'built in Go'])).toBe(false)
  })
})

describe('hasAllTags', () => {
  it('CombinesChosenTagsWithAnd', () => {
    expect(hasAllTags(['go', 'sql'], ['go', 'sql'])).toBe(true)
    expect(hasAllTags(['go'], ['go', 'sql'])).toBe(false)
    expect(hasAllTags([], [])).toBe(true)
  })
})

describe('allTags', () => {
  it('ReturnsDistinctSortedTags', () => {
    expect(allTags([{ tags: ['sql', 'go'] }, { tags: ['go'] }])).toEqual(['go', 'sql'])
  })
})
