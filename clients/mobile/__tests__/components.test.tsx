import React from 'react';
import {Text, TextInput} from 'react-native';
import renderer, {act, ReactTestRenderer} from 'react-test-renderer';
import {Path} from 'react-native-svg';

import {ConfirmDialog} from '../src/components/ConfirmDialog';
import {
  ConflictsIcon,
  FolderIcon,
  OkIcon,
} from '../src/components/icons';
import {Meter, StatusMark, Working} from '../src/components/pixel';
import {icons} from '../src/vendor/nithron-icons';
import {iconSide} from '../src/theme/pixel';
import {palettes} from '../src/theme/theme';

function render(element: React.ReactElement): ReactTestRenderer {
  let tree: ReactTestRenderer | undefined;
  act(() => {
    tree = renderer.create(element);
  });
  return tree!;
}

const INK = palettes.light.ink;

afterEach(() => {
  jest.useRealTimers();
});

describe('icons', () => {
  it("draws the set's path on its 16 by 16 grid, in one colour", () => {
    const tree = render(<FolderIcon color={INK} />);
    const path = tree.root.findByType(Path);
    expect(path.props.d).toBe(icons.folder.path);
    expect(path.props.fill).toBe(INK);
    const svg = tree.root.findByProps({viewBox: '0 0 16 16'});
    expect(svg.props.width).toBe(iconSide(16));
    expect(svg.props.shapeRendering).toBe('crispEdges');
  });

  it('shows 32 px only when asked', () => {
    const tree = render(<OkIcon color={INK} size={32} />);
    const svg = tree.root.findByProps({viewBox: '0 0 16 16'});
    expect(svg.props.width).toBe(iconSide(32));
  });

  it('draws nothing for a meaning without a drawing, so its words stand alone', () => {
    const tree = render(<ConflictsIcon color={INK} />);
    expect(tree.toJSON()).toBeNull();
    expect(ConflictsIcon.drawn).toBe(false);
  });
});

describe('StatusMark', () => {
  it('is a mark and a word in the state colour', () => {
    const tree = render(<StatusMark state="failed">Failed</StatusMark>);
    expect(tree.root.findByType(Path).props.d).toBe(icons.failed.path);
    expect(tree.root.findByType(Path).props.fill).toBe(palettes.light.failed);
    const word = tree.root.findByType(Text);
    expect(word.props.children).toBe('Failed');
  });

  it('blinks a running lamp in steps, and never hides it', () => {
    jest.useFakeTimers();
    const tree = render(<StatusMark state="running">Running</StatusMark>);
    const lamp = () =>
      tree.root.findAll(n => n.props.style?.opacity !== undefined)[0];
    expect(lamp().props.style.opacity).toBe(1);
    act(() => {
      jest.advanceTimersByTime(1800);
    });
    expect(lamp().props.style.opacity).toBeGreaterThan(0);
    expect(lamp().props.style.opacity).toBeLessThan(1);
    act(() => tree.unmount());
  });
});

describe('Meter', () => {
  it('fills a block only when the measure covers it whole', () => {
    const tree = render(
      <Meter value={45} label="Upload" kind="progress" blocks={10} />,
    );
    const bar = tree.root.findByProps({accessibilityRole: 'progressbar'});
    expect(bar.props.accessibilityValue).toEqual({
      min: 0,
      max: 100,
      now: 45,
      text: '45%',
    });
    const flat = bar.props.children as React.ReactElement[];
    expect(flat).toHaveLength(10);
    const filled = flat.filter(b =>
      JSON.stringify(b.props.style).includes(palettes.light.ink),
    );
    expect(filled).toHaveLength(4);
  });
});

describe('Working', () => {
  it('says what it waits for, with a cursor that blinks', () => {
    jest.useFakeTimers();
    const tree = render(<Working>Loading the folder</Working>);
    expect(
      tree.root.findByProps({accessibilityLabel: 'Loading the folder'}),
    ).toBeDefined();
    act(() => tree.unmount());
  });
});

describe('ConfirmDialog', () => {
  const erase = jest.fn();

  function dialog() {
    return render(
      <ConfirmDialog
        visible
        title="Delete report.pdf?"
        message="The file is erased from the share on nOS."
        confirmName="report.pdf"
        actions={[{label: 'Delete report.pdf', onPress: erase}]}
        onCancel={jest.fn()}
      />,
    );
  }

  function buttons(tree: ReactTestRenderer) {
    return tree.root.findAll(
      n => n.props.accessibilityRole === 'button' && n.props.accessibilityLabel,
      {deep: true},
    ).filter((n, i, all) => all.findIndex(m => m.props.accessibilityLabel === n.props.accessibilityLabel) === i);
  }

  it('puts Cancel first and names the action in its button', () => {
    jest.useFakeTimers();
    const tree = dialog();
    const labels = buttons(tree).map(b => b.props.accessibilityLabel);
    expect(labels).toEqual(['Cancel', 'Delete report.pdf']);
    act(() => tree.unmount());
  });

  it('waits for the name to be typed before it erases', () => {
    jest.useFakeTimers();
    const tree = dialog();
    const action = () =>
      buttons(tree).find(
        b => b.props.accessibilityLabel === 'Delete report.pdf',
      )!;
    expect(action().props.accessibilityState).toEqual({disabled: true});
    act(() => {
      tree.root.findByType(TextInput).props.onChangeText('report.pdf');
    });
    expect(action().props.accessibilityState).toEqual({disabled: false});
    act(() => tree.unmount());
  });
});
